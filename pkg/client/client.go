// Package client 是 nazhi-cli SDK 的根包。
//
// 每个 Client 实例拥有独立的 HTTP cookie jar，天然并发安全。
// 所有方法都需要 context.Context，支持超时与取消。
package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

// ─── Client ───

// Client 是目标平台 API 的完整 Go SDK。
// 每个实例拥有独立的 cookie jar，天然并发安全。
type Client struct {
	ssoBaseURL string // SSO 根地址
	baseURL    string // 业务 API 根地址（port 8280）
	// baseURLParsed 为预解析结果：atomic.Pointer 实现 lock-free 读 + CAS 懒解析写入。
	// 之前用 *url.URL + sync.Mutex 仍有 race detector 报警（jar.SetCookies 读 url 字段时与另一 goroutine 的 url.Parse 写 url 字段同步缺失），
	// 改 atomic.Pointer 后所有访问原子化，go test -race 不再报警。
	baseURLParsed atomic.Pointer[url.URL]
	uploadURL     string       // 文件上传服务器地址
	http          *http.Client // 独立 cookie jar
	logger        *slog.Logger
	pendingToken  string // 延迟注入的 X-Auth-Token，New() 末尾统一 syncCookieToken

	// submittedPageSize 是 GetSubmittedCircles 每页请求条数。
	// 默认 defaultSubmittedPageSize（500），服务端 pageSize 上限 500。
	// 通过 WithSubmittedPageSize 可配置。
	submittedPageSize int

	// sm 管理业务 session 的激活状态机（4 步 HAR 激活、backoff 缓存、持锁 fast path）。
	sm *sessionManager
}

// ─── Option 模式 ───

// Option 是 Client 构造函数的选项函数。
type Option func(*Client)

// withURLGuard 生成字符串型 Option 的守卫工厂，消除 WithSSOBase / WithBaseURL /
// WithUploadURL / WithToken 中重复的空字符串守卫 + warn 模式。
//
// 返回 func(string) Option：
//   - v 为空或纯空白：warn 并拒绝设置，保持当前值
//   - 否则：TrimSpace 并去掉尾部斜杠后调用 setter（拼接点均为 "/path" 形态，
//     尾斜杠会产生 //path 双斜杠路径，个别 nginx 配置下 404 且难排查）
func withURLGuard(name string, setter func(*Client, string)) func(string) Option {
	return func(v string) Option {
		if strings.TrimSpace(v) == "" {
			return func(c *Client) {
				c.logger.Warn(name + ": 空字符串被拒绝，保持当前值")
			}
		}
		return func(c *Client) {
			setter(c, strings.TrimRight(strings.TrimSpace(v), "/"))
		}
	}
}

// withNilGuard 生成指针/接口型 Option 的守卫工厂，消除 WithHTTPClient /
// WithLogger / WithHTTPClient 中重复的 nil 守卫 + warn 模式。
//
// 返回 func(T) Option：
//   - v 为 nil：warn 并拒绝设置，保持当前值
//   - 否则：调用 setter
func withNilGuard[T any](name string, setter func(*Client, T)) func(T) Option {
	return func(v T) Option {
		if isNil(v) {
			return func(c *Client) {
				c.logger.Warn(name + ": nil 被拒绝，保持当前值")
			}
		}
		return func(c *Client) {
			setter(c, v)
		}
	}
}

// isNil 通用 nil 检查：支持指针、接口、map、slice、chan、func。
func isNil(v any) bool {
	if v == nil {
		return true
	}
	switch rv := reflect.ValueOf(v); rv.Kind() { //nolint:exhaustive
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

// ─── Option 构造器 ───

// WithSSOBase 设置 SSO 根地址（用于 login 流程）。
// 默认值由 defaultSSOBase 常量提供（见 request.go defaultSSOBase）。
var WithSSOBase = withURLGuard("WithSSOBase", func(c *Client, v string) { c.ssoBaseURL = v })

// WithBaseURL 设置业务 API 根地址。
var WithBaseURL = withURLGuard("WithBaseURL", func(c *Client, v string) { c.baseURL = v })

// WithUploadURL 设置文件上传服务器地址。
var WithUploadURL = withURLGuard("WithUploadURL", func(c *Client, v string) { c.uploadURL = v })

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
var WithLogger = withNilGuard[*slog.Logger]("WithLogger", func(c *Client, l *slog.Logger) { c.logger = l })

// WithHTTPClient 设置自定义 HTTP 客户端（完全替换默认客户端）。
// 注意：替换后 cookie jar 由调用者负责。
//
// 行为约定：
//   - hc == nil：拒绝设置并 warn，保持当前 c.http（防止 nil 静默覆盖
//     默认带 cookie jar 的客户端，导致后续请求 0 cookie → 空 dataList）
//   - 否则：完全替换 c.http；若新客户端 Timeout 为零且此前已通过
//     WithTimeout 设置过超时，则继承该超时——消除 Option 声明顺序敏感性，
//     WithTimeout(15s) 与 WithHTTPClient(custom) 先后不再影响最终生效值
var WithHTTPClient = withNilGuard[*http.Client]("WithHTTPClient", func(c *Client, hc *http.Client) {
	prevTimeout := time.Duration(0)
	if c.http != nil {
		prevTimeout = c.http.Timeout
	}
	c.http = hc
	if hc.Timeout == 0 && prevTimeout > 0 {
		hc.Timeout = prevTimeout
	}
})

// WithToken 预置 X-Auth-Token（同时写入 Header 和 Cookie）。
//
// 用于不经过 Login() 流程、直接从外部传入 token 的场景：
//   - CLI 命令的 --token 标志
//   - 从文件/CI secret 读取的存量 token
//
// 业务服务器要求 X-Auth-Token 同时存在于 Header 和 Cookie，
// 仅设置 Header 会导致后续接口返回空数据。
//
// 行为约定：
//   - token 空字符串或纯空白：拒绝设置并 warn（同 withURLGuard 约束）
//   - 否则：存到 c.pendingToken，延迟到 New() 末尾统一 syncCookieToken
//
// 注意：实际 cookie 注入延迟到 New() 末尾执行，确保 WithSSOBase / WithBaseURL /
// WithHTTPClient 在 WithToken 之后调用也能正确生效（避免 Option 顺序敏感性 bug）。
var WithToken = withURLGuard("WithToken", func(c *Client, v string) { c.pendingToken = v })

// WithSubmittedPageSize 配置 GetSubmittedCircles 每页请求条数。
//
// 行为约定：
//   - n <= 0：拒绝设置并 warn，保持当前值（防止清零或负数）
//   - n > 0: 设置每页条数
//
// 服务端 pageSize 上限 500（实测 pageSize=10000 被截断为 500）。
// 默认值 defaultSubmittedPageSize（500）在绝大多数学期能单页覆盖所有记录，超出时由 fetchAllCirclePages 自动翻页合并。
func WithSubmittedPageSize(n int) Option {
	return func(c *Client) {
		if n <= 0 {
			c.logger.Warn("WithSubmittedPageSize: 非正数被拒绝，保持当前值",
				"current", c.submittedPageSize, "rejected", n)
			return
		}
		c.submittedPageSize = n
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
// 登录走五育活动端免验证码接口（/uiActivityLogin/studentLogin），
// 密码本地 MD5 计算，无需验证码识别器。
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
		ssoBaseURL:        defaultSSOBase,
		baseURL:           defaultBaseURL,
		uploadURL:         defaultUploadURL,
		http:              newHTTPClient(),
		logger:            slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		sm:                &sessionManager{},
		submittedPageSize: defaultSubmittedPageSize,
	}
	for _, opt := range opts {
		opt(c)
	}
	// 所有 Options 跑完后预解析 baseURL 并统一注入 cookie
	// 预解析必须在 syncCookieToken 之前，以免 syncCookieToken 懒解析报错
	if parsed, err := url.Parse(c.baseURL); err == nil {
		c.baseURLParsed.Store(parsed)
	} else {
		c.logger.Warn("New: 预解析 baseURL 失败", "baseURL", c.baseURL, "error", err)
	}
	// 明文 HTTP 告警（cli #10，P2 升级）：业务 API 与上传服务器默认 http:// 明文，
	// 且携带 X-Auth-Token——局域网/ISP 中间人可直接截获 token 与 userName 等 PII。
	// 厂商固定端点不支持 HTTPS 是客观约束，但零告警使该风险完全不可见（且 README
	// 声称有「New 非 HTTPS WARN」，此前并无落地实现）。这里对 baseURL 与 uploadURL
	// 各输出一次 Warn，让调用方在启动期即感知。
	for name, raw := range map[string]string{"baseURL": c.baseURL, "uploadURL": c.uploadURL} {
		if u, err := url.Parse(raw); err == nil && u.Scheme == "http" {
			c.logger.Warn("New: 目标服务使用明文 HTTP（建议 HTTPS，中间人可截获 token 与账号信息）",
				"component", name, "url", raw)
		}
	}
	if c.pendingToken != "" {
		if err := c.syncCookieToken(c.pendingToken); err != nil {
			return c, err // 仍返回 c 让调用方能 Close() 清理资源，但 error 必须 propagate
		}
	}
	return c, nil
}

// ─── 内部辅助 ───

// bizURL 拼接业务 API 完整 URL。
// 用 helper 统一管理 baseURL + path 拼接，
// 避免 5+ 处生产代码裸用 c.baseURL + path。
func (c *Client) bizURL(path string) string {
	return c.baseURL + path
}

// logWithLevel 是结构化日志统一出口，携带 trace_id 并做敏感脱敏。
func (c *Client) logWithLevel(ctx context.Context, lvl slog.Level, format string, args ...any) {
	if c.logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !c.logger.Enabled(ctx, lvl) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	msg = logx.RedactBody(msg)
	if tid := logx.TraceIDFrom(ctx); tid != "" {
		c.logger.Log(ctx, lvl, msg, slog.String("trace_id", tid))
	} else {
		c.logger.Log(ctx, lvl, msg)
	}
}

// logDebug 输出 debug 日志（通过 slog Debug 级别）。
//
// 先 fmt.Sprintf 插值再交 slog，避免格式串被当 key-value 对输出。
//
//   - nil logger 静默返回，避免 nil panic
//   - LevelEnabled 提前检查，非 Debug 级别时跳过 fmt.Sprintf 分配
//     （OCR 热路径会反复调用此函数，无 debug 级别时应避免无谓格式化分配）
func (c *Client) logDebug(format string, args ...any) {
	c.logWithLevel(context.Background(), slog.LevelDebug, format, args...)
}

// logDebugCtx 是携带 context 的 debug 日志，用于透传 trace_id。
func (c *Client) logDebugCtx(ctx context.Context, format string, args ...any) {
	c.logWithLevel(ctx, slog.LevelDebug, format, args...)
}

// LogDebugForTest 暴露给白盒测试的 debug 入口（携带 ctx）。
func (c *Client) LogDebugForTest(ctx context.Context, format string, args ...any) {
	c.logWithLevel(ctx, slog.LevelDebug, format, args...)
}

// LogInfoForTest 暴露给白盒测试的 info 入口（携带 ctx）。
func (c *Client) LogInfoForTest(ctx context.Context, format string, args ...any) {
	c.logWithLevel(ctx, slog.LevelInfo, format, args...)
}

// ─── 资源释放 ───

// Enabled 暴露 logger 的 Enabled 供测试校验级别（不影响生产行为）。
func (c *Client) Enabled(ctx context.Context, lvl slog.Level) bool {
	if c.logger == nil {
		return false
	}
	return c.logger.Enabled(ctx, lvl)
}

// Close 释放 Client 持有的资源：
//   - HTTP Transport 的空闲 keep-alive 连接
//   - sessionManager backoff 状态
func (c *Client) Close() error {
	var errs []error
	if c.http != nil {
		if t, ok := c.http.Transport.(*http.Transport); ok && t != nil {
			t.CloseIdleConnections()
		}
	}
	if c.sm != nil {
		c.sm.Reset()
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
