package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// CaptchaRecognizer 由 build tag 决定：
//   - !ddddocr: nil 默认（见 client_ocr_disabled.go），调用方必须 WithCustomOCR
//   - ddddocr:  ocr.NewPool(0) 默认（见 client_ocr_enabled.go）

// CaptchaRecognizer 是验证码识别器接口。
// *ocr.Pool 实现了该接口，测试时可注入 mock。
type CaptchaRecognizer interface {
	Recognize([]byte) (string, error)
	// Close 释放识别器占用的资源 (ONNX session + 临时目录)。
	// 默认 *ocr.Pool 已实现; mock 必须实现。
	Close() error
}

// ─── Client ───

// Client 是目标平台 API 的完整 Go SDK。
// 每个实例拥有独立的 cookie jar，天然并发安全。
//
// session 激活状态机已提取到 sessionManager，不再直接持有
// sessionToken / sessionMu / lastErr（现为 sm.lastErr） 等字段。
type Client struct {
	ssoBaseURL    string       // SSO 根地址
	baseURL       string       // 业务 API 根地址（port 8280）
	baseURLParsed *url.URL     // baseURL 预解析结果，F6: 避免每次 syncCookieToken 重复 url.Parse
	uploadURL     string       // 文件上传服务器地址
	http          *http.Client // 独立 cookie jar
	logger       *slog.Logger
	ocr          CaptchaRecognizer // 验证码识别器（默认启用进程级 OCR 单例）
	pendingToken string            // 延迟注入的 X-Auth-Token，New() 末尾统一 syncCookieToken

	// sm 管理业务 session 的激活状态机（4 步 HAR 激活、backoff 缓存、DCL fast path）。
	sm *sessionManager

	// cleanTransportInit 保证 clonedTransport 只 Clone 一次。
	// 解决 B1：原实现每次 UploadFile 都 t.Clone() → 50 张图 50 次完整 DNS+TCP+TLS
	// 握手（每次 Clone 出独立对象，丢失累加的 idle 连接池，keep-alive 失效）。
	// 修复后首次 Clone 缓存，后续复用同一 Transport 实例，clean idle 池跨上传累积。
	cleanTransportInit      sync.Once
	cleanTransport          *http.Transport }

// ─── Option 模式 ───

// Option 是 Client 构造函数的选项函数。
type Option func(*Client)

// WithSSOBase 设置 SSO 根地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 ssoBaseURL（防止空字符串
//     静默覆盖 New() 已设的 defaultSSOBase，导致 SSO 拼接出畸形 URL）
//   - 否则：设置 ssoBaseURL
var WithSSOBase = func(url string) Option {
	return func(c *Client) {
		if url == "" {
			c.logger.Warn("WithSSOBase: 空字符串被拒绝，保持当前值", "current", c.ssoBaseURL)
			return
		}
		c.ssoBaseURL = url
	}
}

// WithBaseURL 设置业务 API 根地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 baseURL（防止空字符串
//     静默覆盖 New() 已设的 defaultBaseURL，导致 biz 拼接出畸形 URL）
//   - 否则：设置 baseURL
var WithBaseURL = func(url string) Option {
	return func(c *Client) {
		if url == "" {
			c.logger.Warn("WithBaseURL: 空字符串被拒绝，保持当前值", "current", c.baseURL)
			return
		}
		c.baseURL = url
	}
}

// WithUploadURL 设置文件上传服务器地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 uploadURL（防止空字符串
//     静默覆盖 New() 已设的 defaultUploadURL，导致上传拼接出畸形 URL）
//   - 否则：设置 uploadURL
var WithUploadURL = func(url string) Option {
	return func(c *Client) {
		if url == "" {
			c.logger.Warn("WithUploadURL: 空字符串被拒绝，保持当前值", "current", c.uploadURL)
			return
		}
		c.uploadURL = url
	}
}

// withDurationGuard 生成 Duration 型 Option 的守卫工厂。
// 与 withURLGuard 对称，消除 WithTimeout / WithSessionBackoff 中重复的 d<0 / d==0 守卫。
//
// 返回 func(d time.Duration) Option：
//   - d < 0：warn 并拒绝设置，保持当前值
//   - d == 0：warn 并拒绝设置（防止静默清零），保持当前值
//   - d > 0：调用 setter(c, d)
//
// 调用方负责在返回的 Option 中叠加额外守卫（如 c.http == nil 检查）。
func withDurationGuard(name string, setter func(*Client, time.Duration)) func(time.Duration) Option {
	return func(d time.Duration) Option {
		if d < 0 {
			return func(c *Client) {
				c.logger.Warn(name+": 负数 duration 被拒绝，保持当前值",
					"duration", d)
			}
		}
		if d == 0 {
			return func(c *Client) {
				c.logger.Warn(name + ": 0 duration 被拒绝（防止静默清零），保持当前值")
			}
		}
		return func(c *Client) {
			setter(c, d)
		}
	}
}

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
	base := withDurationGuard("WithTimeout", func(c *Client, v time.Duration) { c.http.Timeout = v })(d)
	return func(c *Client) {
		if c.http == nil {
			c.logger.Warn("WithTimeout: c.http 为 nil，跳过设置",
				"tip", "确保在 WithTimeout 之前未传入 WithHTTPClient(nil)")
			return
		}
		base(c)
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
//   - d > 0：设置 c.sm.backoff
//   - d = 0：拒绝并 warn，保持当前值（防止静默清零已有配置）
//   - d < 0：拒绝并 warn，保持当前值（负数 time.Duration 无意义）
//
// 设计一致：与 WithTimeout 的「d<=0 拒绝 + warn」守卫对称。
//
// 与 ErrSessionBackoff 哨兵配对，
// 让 SDK 用户能调整 thundering herd 抑制窗口。
var WithSessionBackoff = withDurationGuard("WithSessionBackoff",
	func(c *Client, d time.Duration) { c.sm.SetBackoff(d) },
)

// WithLogger 设置自定义 logger。
//
// 行为约定：
//   - l == nil：拒绝设置并 warn，保持当前 logger（防止 nil 覆盖后
//     后续 c.logger.Warn/Debug/Error 全部 nil pointer panic）
//   - 否则：替换 logger
//
// 设计一致：与 WithHTTPClient nil 守卫对称（风格）。
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		if l == nil {
			c.logger.Warn("WithLogger: nil logger 被拒绝，保持当前值",
				"tip", "用 slog.New(slog.NewTextHandler(...)) 创建自定义 logger")
			return
		}
		c.logger = l
	}
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
//
// 行为约定：
//   - r == nil：拒绝设置并 warn，保持当前值（防止 nil 静默覆盖
//     已注入的识别器，导致后续 Login 返回 ErrOCRNotConfigured）
//   - 否则：替换识别器
//
// 设计一致：与 WithLogger(nil) / WithHTTPClient(nil) 的 nil 拒绝守卫对称。
func WithCustomOCR(r CaptchaRecognizer) Option {
	return func(c *Client) {
		if r == nil {
			c.logger.Warn("WithCustomOCR: nil recognizer 被拒绝，保持当前值",
				"tip", "使用真正的 CaptchaRecognizer 实现，或省略本 Option 使用默认 OCR")
			return
		}
		c.ocr = r
	}
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
// （避免业务接口返回空 dataList 但根因在 build client 阶段
// 静默 Warn，跨多步调用难关联）。
func New(opts ...Option) (*Client, error) {
	c := &Client{
		ssoBaseURL: defaultSSOBase,
		baseURL:    defaultBaseURL,
		uploadURL:  defaultUploadURL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		ocr:        defaultOCR(), // build tag 决定：!ddddocr → nil, ddddocr → ocr.NewPool(0)
		sm:         &sessionManager{},
	}
	for _, opt := range opts {
		opt(c)
	}
	// 所有 Options 跑完后预解析 baseURL（F6）并统一注入 cookie
	// 预解析必须在 syncCookieToken 之前，以免 syncCookieToken 懒解析报错
	if parsed, err := url.Parse(c.baseURL); err == nil {
		c.baseURLParsed = parsed
	}
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
// 用 fmt.Sprintf 先格式化再传给 slog。
// 原实现直接 c.logger.Debug(format, args...) 被 slog 当成 key-value 对，
// 不会做 %s/%d 插值，导致日志输出原始的格式字符串而非插值结果。
//
// - nil logger 静默返回，避免 nil panic
// - LevelEnabled 提前检查，非 Debug 级别时跳过 fmt.Sprintf 分配
// OCR 99 张图 × 5 个 logDebug = 500+ 次浪费的格式字符串分配。
func (c *Client) logDebug(format string, args ...any) {
	if c.logger == nil {
		return
	}
	if !c.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	c.logger.Debug(fmt.Sprintf(format, args...))
}

// logSafeBody 截断 bytes 到 100 字符用于日志，防止敏感信息泄露。
func logSafeBody(body []byte) string {
	s := string(body)
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// safeOCRRecognize 调用 c.ocr.Recognize 并 recover panic，转换为 error。
//
// Recognize 实现可能在不可预见的边界条件下
// panic（如 mock 实现有 bug、CGO 层崩溃），如果 panic 不处理会 crash 整个进程。
// safeOCRRecognize 包装 Recognize 调用，捕获 panic 并返回 ErrOCRPanic 哨兵。
//
// 注意：c.ocr 为 nil 时直接返回错误（避免 nil deref），而非默默 success。
func (c *Client) safeOCRRecognize(imgBytes []byte) (text string, err error) {
	if c.ocr == nil {
		return "", ErrOCRNotConfigured
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrOCRPanic, r)
		}
	}()
	return c.ocr.Recognize(imgBytes)
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
	// （保留隔离语义：只关闭 clean client 自己的 idle 池，
	//  不殃及业务 Client 到 sso/api 主机的 keep-alive 连接）
	if c.cleanTransport != nil {
		c.cleanTransport.CloseIdleConnections()
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
