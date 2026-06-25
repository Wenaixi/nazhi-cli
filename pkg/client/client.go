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

	"github.com/Wenaixi/nazhi-cli/internal/ocr"
)

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
}

// ─── Option 模式 ───

// Option 是 Client 构造函数的选项函数。
type Option func(*Client)

// WithSSOBase 设置 SSO 根地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 ssoBaseURL（防止空字符串
//     静默覆盖 New() 已设的 defaultSSOBase，导致 ssoURL() 拼接出畸形 URL）
//   - 否则：设置 ssoBaseURL
//
// 设计一致：与 WithTimeout 一样是「runtime degraded-warn + 不修改字段」，
// 而非 build-time fail-fast——保持与 F8/F9 修复的 Option 校验风格一致。
func WithSSOBase(url string) Option {
	return func(c *Client) {
		if url == "" {
			c.logger.Warn("WithSSOBase: 空字符串被拒绝，保持当前值",
				"current", c.ssoBaseURL)
			return
		}
		c.ssoBaseURL = url
	}
}

// WithBaseURL 设置业务 API 根地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 baseURL（防止空字符串
//     静默覆盖 New() 已设的 defaultBaseURL，导致 bizURL() 拼出畸形 URL）
//   - 否则：设置 baseURL
//
// 设计一致：与 WithSSOBase / WithTimeout 同款 warn + 不修改字段守卫。
func WithBaseURL(url string) Option {
	return func(c *Client) {
		if url == "" {
			c.logger.Warn("WithBaseURL: 空字符串被拒绝，保持当前值",
				"current", c.baseURL)
			return
		}
		c.baseURL = url
	}
}

// WithUploadURL 设置文件上传服务器地址。
//
// 行为约定：
//   - url == ""：拒绝设置并 warn，保持当前 uploadURL（防止空字符串
//     静默覆盖 New() 已设的 defaultUploadURL，导致 uploadServiceURL() 拼出畸形 URL）
//   - 否则：设置 uploadURL
//
// 设计一致：与 WithSSOBase / WithBaseURL 同款 warn + 不修改字段守卫。
func WithUploadURL(url string) Option {
	return func(c *Client) {
		if url == "" {
			c.logger.Warn("WithUploadURL: 空字符串被拒绝，保持当前值",
				"current", c.uploadURL)
			return
		}
		c.uploadURL = url
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
func WithOCRConcurrency(n int) Option {
	return func(c *Client) {
		if n < 0 {
			c.logger.Warn("WithOCRConcurrency: 负数被拒绝，保持当前 OCR 实例",
				"n", n,
				"tip", "用 0/1 = 单实例，N>1 = 并发池")
			return
		}
		c.ocr = ocr.NewPool(n)
	}
}

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
		c.pendingToken = token
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
		ocr:        ocr.NewPool(0), // 默认懒加载单实例（兼容原行为）
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
func (c *Client) logDebug(format string, args ...any) {
	c.logger.Debug(format, args...)
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
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
