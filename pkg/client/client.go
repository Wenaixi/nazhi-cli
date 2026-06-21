package client

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/internal/ocr"
)

// ─── Client ───

// Client 是目标平台 API 的完整 Go SDK。
// 每个实例拥有独立的 cookie jar，天然并发安全。
type Client struct {
	ssoBaseURL string       // SSO 根地址
	baseURL    string       // 业务 API 根地址（port 8280）
	uploadURL  string       // 文件上传服务器地址
	http       *http.Client // 独立 cookie jar
	logger     *slog.Logger
	ocr        *ocr.OCR     // 验证码识别器（可选，WithOCR 启用）
}

// ─── Option 模式 ───

// Option 是 Client 构造函数的选项函数。
type Option func(*Client)

// WithSSOBase 设置 SSO 根地址。
func WithSSOBase(url string) Option {
	return func(c *Client) { c.ssoBaseURL = url }
}

// WithBaseURL 设置业务 API 根地址。
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithUploadURL 设置文件上传服务器地址。
func WithUploadURL(url string) Option {
	return func(c *Client) { c.uploadURL = url }
}

// WithTimeout 设置 HTTP 客户端超时（包括连接、TLS 握手、响应体读取）。
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if c.http != nil {
			c.http.Timeout = d
		}
	}
}

// WithLogger 设置自定义 logger。
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) { c.logger = l }
}

// WithHTTPClient 设置自定义 HTTP 客户端（完全替换默认客户端）。
// 注意：替换后 cookie jar 由调用者负责。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

// WithOCR 启用内置 OCR 验证码识别。
// 模型文件已内嵌在二进制中，无需额外下载。
// 启用后 Login 方法在未提供 captcha 时将自动识别验证码图片。
func WithOCR() Option {
	return func(c *Client) {
		c.ocr = ocr.New()
	}
}

// ─── 构造 ───

// New 创建 Client。使用 Option 模式配置：
//
//	client := nazhicli.New(
//	    nazhicli.WithSSOBase("https://www.nazhisoft.com"),
//	    nazhicli.WithTimeout(15*time.Second),
//	)
func New(opts ...Option) *Client {
	c := &Client{
		ssoBaseURL: defaultSSOBase,
		baseURL:    defaultBaseURL,
		uploadURL:  defaultUploadURL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
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
