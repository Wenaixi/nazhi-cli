package client

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// captchaRecognizer 由 build tag 决定：
//   - !ddddocr: nil 默认（见 client_ocr_disabled.go），调用方必须 WithCustomOCR
//   - ddddocr:  ocr.NewPool(0) 默认（见 client_ocr_enabled.go）

// captchaRecognizer 是验证码识别器接口。
// *ocr.Pool 实现了该接口，测试时可注入 mock。
type captchaRecognizer interface {
	Recognize([]byte) (string, error)
	// Close 释放识别器占用的资源 (ONNX session + 临时目录)。
	// 默认 *ocr.Pool 已实现; mock 必须实现。
	Close() error
}

// ─── Client ───

// Client 是目标平台 API 的完整 Go SDK。
// 每个实例拥有独立的 cookie jar，天然并发安全。
type Client struct {
	ssoBaseURL   string       // SSO 根地址
	baseURL      string       // 业务 API 根地址（port 8280）
	uploadURL    string       // 文件上传服务器地址
	http         *http.Client // 独立 cookie jar
	logger       *slog.Logger
	ocr          captchaRecognizer // 验证码识别器（默认启用进程级 OCR 单例）
	pendingToken string            // 延迟注入的 X-Auth-Token，New() 末尾统一 syncCookieToken

	// sessionToken 记录上次成功激活业务 session 的 token。
	// sessionMu 保护 sessionToken 的并发读写。
	// 解决了原 sync.Once 不感知 token 变更的问题：进程内 token 变化
	//（如重新 Login）时重新执行 4 步激活，确保 cookie jar 中的 session
	// cookie 与当前 token 一致，避免后续业务接口返回空数据。
	sessionToken string
	sessionMu    sync.Mutex

	// sessionBackoff 控制激活失败后重试的最小间隔。
	// 默认 0 表示使用内部默认值（5 秒）；测试可设为较大值以保证
	// 所有并发 goroutine 都触达 backoff 窗口。
	sessionBackoff time.Duration

	// lastActivationErr、lastAttemptAt 和 lastFailedToken 构成激活失败缓存。
	// 当激活失败时记录错误、时间戳和失败的 token，后续 goroutine 在 backoff
	// 窗口内且 token 相同时直接返回缓存错误，避免 thundering herd 重试放大。
	//
	// F15 修复（round-7）：缓存键必须包含 token 维度。同一 Client 切换 token
	// 重新激活时（如 token 过期换新 token），新 token 不应被旧 token 的失败
	// 缓存抑制 — 否则会返回 stale error 而不实际尝试新 token 激活。
	lastActivationErr error
	lastAttemptAt     time.Time
	lastFailedToken   string

	// cleanTransportInit 保证 clonedTransport 只 Clone 一次。
	// 解决 B1：原实现每次 UploadFile 都 t.Clone() → 50 张图 50 次完整 DNS+TCP+TLS
	// 握手（每次 Clone 出独立对象，丢失累加的 idle 连接池，keep-alive 失效）。
	// 修复后首次 Clone 缓存，后续复用同一 Transport 实例，clean idle 池跨上传累积。
	cleanTransportInit sync.Once
	cleanTransport     *http.Transport // 懒加载的 cloned Transport（仅 *http.Transport 路径）

	// cachedUserInfo 缓存步骤 4 获取的 UserInfo（B10 修复），供 GetMyInfo 复用。
	cachedUserInfo *types.UserInfo
}

// ─── Option 模式 ───

// Option 是 Client 构造函数的选项函数。
type Option func(*Client)

// withURLGuard 提取 URL 型 Option（WithSSOBase / WithBaseURL / WithUploadURL）
// 的通用守卫逻辑。
//
// 三者 guard 模式完全相同：空字符串 → warn + 保留原值；非空 → setter(c, url)。
// name 用于 warn 消息前缀；getter 获取字段当前值（warn 时输出）；setter 负责赋值。
func withURLGuard(name string, getter func(*Client) string, setter func(*Client, string)) func(string) Option {
	return func(url string) Option {
		return func(c *Client) {
			if url == "" {
				c.logger.Warn(name+": 空字符串被拒绝，保持当前值",
					"current", getter(c))
				return
			}
			setter(c, url)
		}
	}
}

// WithSSOBase 设置 SSO 根地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 ssoBaseURL（防止空字符串
//     静默覆盖 New() 已设的 defaultSSOBase，导致 ssoURL() 拼接出畸形 URL）
//   - 否则：设置 ssoBaseURL
//
// 设计一致：与 WithTimeout 一样是「runtime degraded-warn + 不修改字段」，
// 而非 build-time fail-fast——保持与 F8/F9 修复的 Option 校验风格一致。
var WithSSOBase = withURLGuard("WithSSOBase",
	func(c *Client) string { return c.ssoBaseURL },
	func(c *Client, v string) { c.ssoBaseURL = v },
)

// WithBaseURL 设置业务 API 根地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 baseURL（防止空字符串
//     静默覆盖 New() 已设的 defaultBaseURL，导致 bizURL() 拼出畸形 URL）
//   - 否则：设置 baseURL
//
// 设计一致：与 WithSSOBase / WithTimeout 同款 warn + 不修改字段守卫。
var WithBaseURL = withURLGuard("WithBaseURL",
	func(c *Client) string { return c.baseURL },
	func(c *Client, v string) { c.baseURL = v },
)

// WithUploadURL 设置文件上传服务器地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 uploadURL（防止空字符串
//     静默覆盖 New() 已设的 defaultUploadURL，导致 uploadServiceURL() 拼出畸形 URL）
//   - 否则：设置 uploadURL
//
// 设计一致：与 WithSSOBase / WithBaseURL 同款 warn + 不修改字段守卫。
var WithUploadURL = withURLGuard("WithUploadURL",
	func(c *Client) string { return c.uploadURL },
	func(c *Client, v string) { c.uploadURL = v },
)

// WithTimeout 设置 HTTP 客户端超时（包括连接、TLS 握手、响应体读取）。
//
// 行为约定：
//   - c.http == nil：拒绝设置并 warn（外部 WithHTTPClient(nil) 误用，
//     静默 return 会让调用方完全感知不到 timeout 未生效）
//   - d > 0：设置超时
//   - d = 0：拒绝设置并 warn，保持当前 Timeout（防止静默把已有
//     正数超时清零为 net/http 默认"无超时"，请求可能永久挂起）
//   - d < 0：拒绝设置并 warn，保持当前 Timeout（防止意外把超时改小）
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if c.http == nil {
			c.logger.Warn("WithTimeout: c.http 为 nil，跳过设置",
				"tip", "确保在 WithTimeout 之前未传入 WithHTTPClient(nil)")
			return
		}
		if d < 0 {
			c.logger.Warn("WithTimeout: 负数超时被拒绝，保持当前值",
				"duration", d, "current", c.http.Timeout)
			return
		}
		if d == 0 {
			c.logger.Warn("WithTimeout: 0 表示 net/http 默认'无超时'，所有请求可能永久挂起，保持当前值",
				"current", c.http.Timeout,
				"tip", "用 WithTimeout(15*time.Second) 设置正数")
			return
		}
		c.http.Timeout = d
	}
}

// WithSessionBackoff 设置 session 激活失败后抑制重试的时间窗口。
//
// 默认值：5 秒（见 defaultSessionBackoff 常量）。SDK 用户调高/调低本字段
// 可针对不同服务端稳定性做适配：
//   - 高频调用场景：调小到 1s 让失败快速重试
//   - 服务端降级场景：调大到 30s 让瞬时故障不被重复激活放大
//
// 行为约定：
//   - d > 0：设置 c.sessionBackoff
//   - d = 0：拒绝并 warn，保持当前值（防止静默清零已有配置）
//   - d < 0：拒绝并 warn，保持当前值（负数 time.Duration 无意义）
//
// 设计一致：与 WithTimeout 的「d<=0 拒绝 + warn」守卫对称。
//
// F15/H2 修复（round-7/round-9）：与 ErrSessionBackoff 哨兵配对，
// 让 SDK 用户能调整 thundering herd 抑制窗口。
func WithSessionBackoff(d time.Duration) Option {
	return func(c *Client) {
		if d < 0 {
			c.logger.Warn("WithSessionBackoff: 负数 backoff 窗口被拒绝，保持当前值",
				"duration", d, "current", c.sessionBackoff)
			return
		}
		if d == 0 {
			c.logger.Warn("WithSessionBackoff: 0 窗口被拒绝（防止静默清零默认值），保持当前值",
				"current", c.sessionBackoff,
				"tip", "用 WithSessionBackoff(5*time.Second) 设置正数，或保留默认值 5s")
			return
		}
		c.sessionBackoff = d
	}
}

// WithLogger 设置自定义 logger。
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// WithHTTPClient 设置自定义 HTTP 客户端（完全替换默认客户端）。
// 注意：替换后 cookie jar 由调用者负责。
//
// 行为约定：
//   - hc == nil：拒绝设置并 warn，保持当前 c.http（防止 nil 静默覆盖
//     默认带 cookie jar 的客户端，导致后续请求 0 cookie → 空 dataList）
//   - 否则：完全替换 c.http
//
// 设计一致：与 WithTimeout 的「c.http == nil 拒绝」对称守卫。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc == nil {
			c.logger.Warn("WithHTTPClient: nil 客户端被拒绝，保持当前值",
				"tip", "如需禁用 HTTP 客户端请直接调用 Close() 或不构造 Client")
			return
		}
		c.http = hc
	}
}

// WithCustomOCR 是测试用 Option，注入自定义验证码识别器。
// 仅在测试中使用。
func WithCustomOCR(r captchaRecognizer) Option {
	return func(c *Client) { c.ocr = r }
}

// WithOCRConcurrency 设置 OCR 实例池预分配数量。
//
// 行为约定：
//   - 0 或 1 = 默认懒加载单实例（与原单例行为一致，1 路串行识别）
//   - N > 1 = 预分配 N 个 OCR 结构体，ONNX session 惰性初始化，
//     首次调用 Recognize 时触发完整模型加载
//   - n < 0：拒绝设置并 warn，保持当前 c.ocr（防止负数被静默截 0
//     后用默认值覆盖调用方已注入的自定义识别器，如 WithCustomOCR mock）
//
// 内存代价：每个 ONNX session 约 50MB（模型 + 原生库），N=4 约 200MB。
// 业务场景：批量调用 Login() 时才需要调高；单次 Login 用 1 实例足够。
//
// 实现按 build tag 分发：见 client_ocr_enabled.go（ddddocr）和
// client_ocr_disabled.go（!ddddocr — 仅返回 warn 占位实现）。
//
// 函数签名在两个文件中保持一致（(int) Option），保证 Option 接口契约。

// WithToken 预置 X-Auth-Token（同时写入 Header 和 Cookie）。
//
// 用于不经过 Login() 流程、直接从外部传入 token 的场景：
//   - CLI 命令的 --token 标志
//   - 从文件/CI secret 读取的存量 token
//
// 业务服务器要求 X-Auth-Token 同时存在于 Header 和 Cookie（参见 auth-flow.md），
// 仅设置 Header 会导致后续接口返回空数据。
//
// 行为约定：
//   - token 是空字符串或纯空白：拒绝设置并 warn，保持当前 pendingToken
//     （防止空 token 静默覆盖已有有效 token，后续 syncCookieToken 写入空
//     cookie 导致业务鉴权失败）
//   - 否则：存到 c.pendingToken，延迟到 New() 末尾统一 syncCookieToken
//
// 注意：实际 cookie 注入延迟到 New() 末尾执行，确保 WithSSOBase / WithBaseURL /
// WithHTTPClient 在 WithToken 之后调用也能正确生效（避免 Option 顺序敏感性 bug）。
func WithToken(token string) Option {
	return func(c *Client) {
		if strings.TrimSpace(token) == "" {
			c.logger.Warn("WithToken: 空字符串或纯空白 token 被拒绝，保持当前值",
				"current_empty", c.pendingToken == "")
			return
		}
		c.pendingToken = strings.TrimSpace(token)
	}
}

// ─── 构造 ───

// New 创建 Client。使用 Option 模式配置：
//
//	client := nazhicli.New(
//	    nazhicli.WithSSOBase("https://www.nazhisoft.com"),
//	    nazhicli.WithTimeout(15*time.Second),
//	)
//
// OCR 验证码识别器默认启用进程级 Pool 单实例（与原单例行为一致）。
// 批量并发场景可用 WithOCRConcurrency(N) 预热 N 个独立 session 实例。
//
// Option 处理顺序：所有 Options 跑完后，若有 WithToken 注入，则在最终 c.http.Jar /
// c.ssoBaseURL / c.baseURL 已知的前提下统一 syncCookieToken（避免顺序敏感性 bug）。
//
// 返回 error：当 WithHTTPClient 自定义 Jar + WithToken 时，Jar 必须支持 cookie 写入。
// 若 Jar 不是 *cookiejar.Jar，syncCookieToken 会返回 error 让调用方立即感知
// （修复 review-tdd F8：避免业务接口返回空 dataList 但根因在 build client 阶段
// 静默 Warn，跨多步调用难关联）。
func New(opts ...Option) (*Client, error) {
	c := &Client{
		ssoBaseURL: defaultSSOBase,
		baseURL:    defaultBaseURL,
		uploadURL:  defaultUploadURL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		ocr:        defaultOCR(), // build tag 决定：!ddddocr → nil, ddddocr → ocr.NewPool(0)
	}
	for _, opt := range opts {
		opt(c)
	}
	// 所有 Options 跑完后统一注入 cookie（baseURL / Jar 都是最终值）
	if c.pendingToken != "" {
		if err := c.syncCookieToken(c.pendingToken); err != nil {
			return c, err // 仍返回 c 让调用方能 Close() 清理资源，但 error 必须 propagate
		}
	}
	return c, nil
}

// ─── 内部辅助 ───

// logDebug 输出 debug 日志（通过 slog Debug 级别）。
//
// G1 修复（review-tdd round-1）：用 fmt.Sprintf 先格式化再传给 slog。
// 原实现直接 c.logger.Debug(format, args...) 被 slog 当成 key-value 对，
// 不会做 %s/%d 插值，导致日志输出原始的格式字符串而非插值结果。
func (c *Client) logDebug(format string, args ...any) {
	c.logger.Debug(fmt.Sprintf(format, args...))
}

// ssoURL 拼接 SSO 路径。
func (c *Client) ssoURL(path string) string {
	return c.ssoBaseURL + path
}

// bizURL 拼接业务 API 路径。
func (c *Client) bizURL(path string) string {
	return c.baseURL + path
}

// uploadServiceURL 拼接文件上传路径。
func (c *Client) uploadServiceURL(path string) string {
	return c.uploadURL + path
}

// ─── 资源释放 ───

// Close 释放 Client 持有的资源：
//   - 底层 OCR 识别器 (ONNX session + %TEMP%/nazhi-cli-ocr-XXXX/ 临时目录)
//   - HTTP Transport 的空闲 keep-alive 连接 (避免进程退出前留有 half-closed 连接)
//
// 用法: CLI 入口处 defer c.Close(), 让每次执行不留垃圾。
//
// 错误: 聚合所有清理错误返回。常见原因: Windows AV 持锁 / Linux 权限拒绝
// 临时目录; 业务上可以 log 出来警告, 但不应阻塞退出。
func (c *Client) Close() error {
	var errs []error
	if c.ocr != nil {
		if err := c.ocr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭 OCR 识别器: %w", err))
		}
	}
	if c.http != nil {
		if t, ok := c.http.Transport.(*http.Transport); ok && t != nil {
			t.CloseIdleConnections()
		}
	}
	// B1：清理 UploadFile 用的 cached cloned Transport 独立 idle 池
	// （保留 F9 隔离语义：只关闭 clean client 自己的 idle 池，
	//  不殃及业务 Client 到 sso/api 主机的 keep-alive 连接）
	if c.cleanTransport != nil {
		c.cleanTransport.CloseIdleConnections()
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
