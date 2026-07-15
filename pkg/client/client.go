// Package client 是 nazhi-cli SDK 的根包。
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

	"github.com/Wenaixi/nazhi-cli/internal/recoverx"
)

// CaptchaRecognizer 由 build tag 决定：
//   - !ddddocr: nil 默认（见 client_ocr_disabled.go），调用方必须 WithCustomOCR
//   - ddddocr:  ocr.NewPool(0) 默认（见 client_ocr_enabled.go）

// CaptchaRecognizer 是验证码识别器接口。
// *ocr.Pool 实现了该接口，测试时可注入 mock。
//
// 注意：实现必须同时实现 Close() error，因为 Client.Close() 会
// 无条件调用 c.ocr.Close()（见 c.ocr != nil 检查后的路径）。
// 即使实现不做资源清理，Close() 也必须存在且返回 nil。
type CaptchaRecognizer interface {
	Recognize([]byte) (string, error)
	// Close 释放识别器占用的资源 (ONNX session + 临时目录)。
	// 默认 *ocr.Pool 已实现; 所有实现（含 mock）必须提供 Close 方法。
	Close() error
}

// ─── Client ───

// Client 是目标平台 API 的完整 Go SDK。
// 每个实例拥有独立的 cookie jar，天然并发安全。
//
// session 激活状态机已提取到 sessionManager，不再直接持有
// sessionToken / sessionMu / lastErr（现为 sm.lastErr） 等字段。
type Client struct {
	ssoBaseURL string // SSO 根地址
	baseURL    string // 业务 API 根地址（port 8280）
	// baseURLParsed 预解析结果，F6 优化 + F3 修复：atomic.Pointer 实现 lock-free 读 + CAS 懒解析写入。
	// 之前用 *url.URL + sync.Mutex 仍有 race detector 报警（jar.SetCookies 读 url 字段时与另一 goroutine 的 url.Parse 写 url 字段同步缺失），
	// 改 atomic.Pointer 后所有访问原子化，go test -race 不再报警。
	baseURLParsed atomic.Pointer[url.URL]
	uploadURL     string       // 文件上传服务器地址
	http          *http.Client // 独立 cookie jar
	logger        *slog.Logger
	ocr           CaptchaRecognizer // 验证码识别器（默认启用进程级 OCR 单例）
	fallbackOCR   CaptchaRecognizer // 降级验证码识别器（ddddocr），primary 失败时自动 fallback
	fallbackConc  int               // fallback ddddocr 池大小，默认 1，由 WithFallbackConcurrency 配置
	ocrModelDir   string            // 外部 ddddocr 模型目录，非空时 OCR 从该目录加载模型/库文件
	pendingToken  string            // 延迟注入的 X-Auth-Token，New() 末尾统一 syncCookieToken

	// submittedPageSize 是 GetSubmittedCircles 每页请求条数。
	// 默认 100，服务端 pageSize 上限 500。
	// 通过 WithSubmittedPageSize 可配置。
	submittedPageSize int

	// sm 管理业务 session 的激活状态机（4 步 HAR 激活、backoff 缓存、DCL fast path）。
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
//   - 否则：TrimSpace 后调用 setter
func withURLGuard(name string, setter func(*Client, string)) func(string) Option {
	return func(v string) Option {
		if strings.TrimSpace(v) == "" {
			return func(c *Client) {
				c.logger.Warn(name + ": 空字符串被拒绝，保持当前值")
			}
		}
		return func(c *Client) {
			setter(c, strings.TrimSpace(v))
		}
	}
}

// withNilGuard 生成指针/接口型 Option 的守卫工厂，消除 WithHTTPClient /
// WithLogger / WithCustomOCR 中重复的 nil 守卫 + warn 模式。
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

// isNil 安全检查任意值是否为 nil，处理 typed nil 指针/接口。
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() { //nolint:exhaustive
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

// WithSSOBase 设置 SSO 根地址。
var WithSSOBase = withURLGuard("WithSSOBase", func(c *Client, v string) { c.ssoBaseURL = v })

// WithBaseURL 设置业务 API 根地址。
var WithBaseURL = withURLGuard("WithBaseURL", func(c *Client, v string) { c.baseURL = v })

// WithUploadURL 设置文件上传服务器地址。
var WithUploadURL = withURLGuard("WithUploadURL", func(c *Client, v string) { c.uploadURL = v })

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
var WithLogger = withNilGuard[*slog.Logger]("WithLogger", func(c *Client, l *slog.Logger) { c.logger = l })

// WithHTTPClient 设置自定义 HTTP 客户端（完全替换默认客户端）。
// 注意：替换后 cookie jar 由调用者负责。
//
// 行为约定：
//   - hc == nil：拒绝设置并 warn，保持当前 c.http（防止 nil 静默覆盖
//     默认带 cookie jar 的客户端，导致后续请求 0 cookie → 空 dataList）
//   - 否则：完全替换 c.http
var WithHTTPClient = withNilGuard[*http.Client]("WithHTTPClient", func(c *Client, hc *http.Client) { c.http = hc })

// WithCustomOCR 注入自定义验证码识别器。
//
// 适用场景：
//   - 测试时注入 mock 验证码识别器
//   - CGO-free 构建（!ddddocr）时注入外部识别器（如 AI 服务）
//
// 行为约定：
//   - r == nil：拒绝设置并 warn，保持当前值（防止 nil 静默覆盖
//     已注入的识别器，导致后续 Login 返回 ErrOCRNotConfigured）
//   - 否则：替换识别器
var WithCustomOCR = withNilGuard[CaptchaRecognizer]("WithCustomOCR", func(c *Client, r CaptchaRecognizer) { c.ocr = r })

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
// 默认值 100 在绝大多数学期能单页覆盖所有记录，>100 条时自动翻页。
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

// WithFallbackOCR 启用/禁用降级 OCR（ddddocr 兜底）。
//
// 启用时：primary OCR（通过 WithCustomOCR 注入）全部失败后，自动降级到
// 内置 ddddocr 重新识别新图片。降级策略：
//  1. primary 尝试最多 9 张图片
//  2. 全部失败且启用了 fallback → 重新拉 9 张新图给 ddddocr 识别
//  3. ddddocr 也失败 → 返回最终错误
//
// 禁用时：保持原行为，primary 失败直接返回错误，无额外内存开销。
//
// 行为约定：
//   - enabled == true: 将 c.fallbackOCR 设为 defaultOCR()（懒加载 ddddocr Pool）
//   - enabled == false: 清空 c.fallbackOCR（如有 ddddocr Pool 则延迟 GC 回收）
//
// 注意：当构建时未启用 -tags ddddocr，defaultOCR() 返回 nil，
// WithFallbackOCR(true) 仍可用但不产生降级效果（c.fallbackOCR 保持 nil）。
//
// 并发度见 WithFallbackConcurrency。
func WithFallbackOCR(enabled bool) Option {
	return func(c *Client) {
		if enabled {
			poolSize := c.fallbackConc
			if poolSize < 1 {
				poolSize = 1
			}
			// defaultOCR 返回带并发度的 Pool
			c.fallbackOCR = defaultFallbackOCR(poolSize)
		} else {
			c.fallbackOCR = nil
		}
	}
}

// WithFallbackConcurrency 设置 fallback ddddocr 并发池大小。
//
// 默认值 1（单实例，串行识别）。增大到 N 时预分配 N 个 OCR 结构体，
// ONNX session 惰性初始化，首次调用 Recognize 时触发完整模型加载。
//
// 内存代价：每个 ONNX session 约 50MB，N=4 约 200MB。
//
// 行为约定：
//   - n < 1：warn + 保持当前值（与 WithOCRConcurrency 守卫一致）
//   - n >= 1：设置 c.fallbackConc；若 c.fallbackOCR 已是 ddddocr Pool 则重建
//
// 调用顺序：WithFallbackConcurrency 可在 WithFallbackOCR 之前或之后调用。
// 若在之后调用，将重建 fallback ddddocr Pool（保持降级启用状态）。
func WithFallbackConcurrency(n int) Option {
	return func(c *Client) {
		if n < 1 {
			c.logger.Warn("WithFallbackConcurrency: 非正数被拒绝，保持当前值",
				"n", n)
			return
		}
		prev := c.fallbackConc
		c.fallbackConc = n

		// 若 fallback 已启用（c.fallbackOCR 非 nil）则重建 Pool
		if c.fallbackOCR != nil {
			c.fallbackOCR = defaultFallbackOCR(n)
			if c.fallbackOCR == nil && prev < 1 && c.logger != nil {
				c.logger.Warn("WithFallbackConcurrency: 当前构建无 ddddocr（-tags ddddocr 缺失），重建 fallback 后仍是 nil")
			}
		}
	}
}

// WithOCRModelDir 设置外部 ddddocr 模型目录。
//
// 非空时，内置 ddddocr OCR 初始化时从该目录加载模型文件和 ONNX Runtime
// 原生库（.dll/.so/.dylib），而非使用 //go:embed 嵌入的数据。
//
// 适用场景：
//   - 嵌入的模型文件被移除以减小二进制体积
//   - 运行时替换/升级模型版本
//
// 行为约定：
//   - dir 为空字符串：拒绝设置并 warn，保持当前值
//   - dir 非空：存储路径，延迟到 OCR 首次 Recognize 时生效
func WithOCRModelDir(dir string) Option {
	return func(c *Client) {
		if strings.TrimSpace(dir) == "" {
			c.logger.Warn("WithOCRModelDir: 空字符串被拒绝，保持当前值")
			return
		}
		c.ocrModelDir = dir

		// 如果 c.ocr 已经是 ddddocr Pool，且当前没有外部目录设置，则注入
		if pool, ok := c.ocr.(interface{ SetModelDir(string) }); ok {
			pool.SetModelDir(dir)
		}
		// fallback 同理
		if fbPool, ok := c.fallbackOCR.(interface{ SetModelDir(string) }); ok {
			fbPool.SetModelDir(dir)
		}
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
		ssoBaseURL:        defaultSSOBase,
		baseURL:           defaultBaseURL,
		uploadURL:         defaultUploadURL,
		http:              newHTTPClient(),
		logger:            slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		ocr:               defaultOCR(),
		sm:                &sessionManager{},
		submittedPageSize: defaultSubmittedPageSize,
		fallbackConc:      1,
	}
	for _, opt := range opts {
		opt(c)
	}
	// 所有 Options 跑完后传入 OCR 模型目录到 default OCR（如果在 WithOCRModelDir
	// 之后 defaultOCR 才被构造的情况，如 WithOCRModelDir 先于 WithCustomOCR 调用）
	if c.ocrModelDir != "" {
		if pool, ok := c.ocr.(interface{ SetModelDir(string) }); ok {
			pool.SetModelDir(c.ocrModelDir)
		}
		if fbPool, ok := c.fallbackOCR.(interface{ SetModelDir(string) }); ok {
			fbPool.SetModelDir(c.ocrModelDir)
		}
	}
	// 所有 Options 跑完后预解析 baseURL（F6）并统一注入 cookie
	// 预解析必须在 syncCookieToken 之前，以免 syncCookieToken 懒解析报错
	if parsed, err := url.Parse(c.baseURL); err == nil {
		c.baseURLParsed.Store(parsed)
	} else {
		c.logger.Warn("New: 预解析 baseURL 失败", "baseURL", c.baseURL, "error", err)
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

// logDebug 输出 debug 日志（通过 slog Debug 级别）。
//
// 用 fmt.Sprintf 先格式化再传给 slog。
// 原实现直接 c.logger.Debug(format, args...) 被 slog 当成 key-value 对，
// 不会做 %s/%d 插值，导致日志输出原始的格式字符串而非插值结果。
//
//   - nil logger 静默返回，避免 nil panic
//   - LevelEnabled 提前检查，非 Debug 级别时跳过 fmt.Sprintf 分配
//     （OCR 热路径会反复调用此函数，无 debug 级别时应避免无谓格式化分配）
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
		if err2 := recoverx.RecoverPanic(recover(), ErrOCRPanic, "safeOCRRecognize"); err2 != nil {
			err = err2
		}
	}()
	return c.ocr.Recognize(imgBytes)
}

// safeFallbackRecognize 调用 c.fallbackOCR.Recognize 并 recover panic。
// 与 safeOCRRecognize 对称，只是操作 fallback 识别器。
func (c *Client) safeFallbackRecognize(imgBytes []byte) (text string, err error) {
	if c.fallbackOCR == nil {
		return "", ErrOCRNotConfigured
	}
	defer func() {
		if err2 := recoverx.RecoverPanic(recover(), ErrOCRPanic, "safeFallbackRecognize"); err2 != nil {
			err = err2
		}
	}()
	return c.fallbackOCR.Recognize(imgBytes)
}

// ─── 资源释放 ───

// Close 释放 Client 持有的资源：
//   - 底层 OCR 识别器（ONNX session + 临时目录）
//   - 降级 OCR 识别器（如果有）
//   - HTTP Transport 的空闲 keep-alive 连接
//   - sessionManager backoff 状态
func (c *Client) Close() error {
	var errs []error
	if c.ocr != nil {
		if err := c.ocr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭 OCR 识别器: %w", err))
		}
	}
	if c.fallbackOCR != nil {
		if err := c.fallbackOCR.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭 fallback OCR 识别器: %w", err))
		}
	}
	if c.http != nil {
		if t, ok := c.http.Transport.(*http.Transport); ok && t != nil {
			t.CloseIdleConnections()
		}
	}
	if c.sm != nil {
		c.sm.mu.Lock()
		c.sm.clearBackoff()
		c.sm.mu.Unlock()
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
