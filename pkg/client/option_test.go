//go:build !ddddocr
// +build !ddddocr

// option_test.go (!ddddocr 构建) 聚合客户端 Option 的白盒测试：
//   - WithTimeout 守卫：负数/0/nil http
//   - WithToken 守卫：late binding + trim 空白
//   - WithLogger / WithCustomOCR nil 守卫
//   - WithSessionBackoff 正/零/负值处理
//   - WithOCRConcurrency 占位实现行为
package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// mockCaptchaRecognizer 测试用 mock：只记录被设置过、Close 不报错。
type mockCaptchaRecognizer struct {
	closed bool
}

func (m *mockCaptchaRecognizer) Recognize([]byte) (string, error) { return "ok", nil }
func (m *mockCaptchaRecognizer) Close() error                     { m.closed = true; return nil }

// ─── option_guards_noocr_test.go: !ddddocr 占位实现 ───

// TestWithOCRConcurrency_Zero_NoWarn_NoDdddOCR 验证 !ddddocr 构建下 WithOCRConcurrency(0) 静默 no-op。
// 修复：n=0 不应输出 warn（合法降级请求，与 WithTimeout(0) 语义不同）。
func TestWithOCRConcurrency_Zero_NoWarn_NoDdddOCR(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mock := &mockCaptchaRecognizer{}
	c := &Client{
		ocr:    mock, // 模拟 WithCustomOCR(mock) 已注入
		logger: logger,
	}

	WithOCRConcurrency(0)(c)

	if c.ocr != mock {
		t.Errorf("!ddddocr 构建下 WithOCRConcurrency(0) 必须保持 c.ocr 不变，实际被替换")
	}
	if logBuf.Len() > 0 {
		t.Errorf("n=0 不应输出 warn，实际 log：%s", logBuf.String())
	}
}

// TestWithOCRConcurrency_Negative_Warns_NoDdddOCR 验证 !ddddocr 构建下 WithOCRConcurrency(-1) 输出 warn。
func TestWithOCRConcurrency_Negative_Warns_NoDdddOCR(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mock := &mockCaptchaRecognizer{}
	c := &Client{
		ocr:    mock,
		logger: logger,
	}

	WithOCRConcurrency(-1)(c)

	if c.ocr != mock {
		t.Errorf("!ddddocr 构建下 WithOCRConcurrency(-1) 必须保持 c.ocr 不变，实际被替换")
	}
	if !strings.Contains(logBuf.String(), "负数") {
		t.Errorf("n<0 应输出 warn 包含 '负数'，实际 log：%s", logBuf.String())
	}
}

// TestWithOCRConcurrency_Positive_Warns_NoDdddOCR 验证 !ddddocr 构建下 WithOCRConcurrency(2) 输出 warn。
func TestWithOCRConcurrency_Positive_Warns_NoDdddOCR(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mock := &mockCaptchaRecognizer{}
	c := &Client{
		ocr:    mock, // 模拟 WithCustomOCR(mock) 已注入
		logger: logger,
	}

	WithOCRConcurrency(2)(c)

	if c.ocr != mock {
		t.Errorf("!ddddocr 构建下 WithOCRConcurrency 必须保持 c.ocr 不变（占位实现），实际被替换")
	}
	if !strings.Contains(logBuf.String(), "ddddocr") || !strings.Contains(logBuf.String(), "WithOCRConcurrency") {
		t.Errorf("应 warn 包含 'ddddocr' 和 'WithOCRConcurrency'，引导调用方改用 WithCustomOCR。实际 log：%s", logBuf.String())
	}
}

// ─── with_timeout_test.go: WithTimeout 守卫 ───

// TestWithTimeout_NegativeRejected 回归测试：WithTimeout(-1) 必须被拒绝，
// 保持当前 Timeout 值（防止把超时改成无效负数）。
// 历史 bug：WithTimeout 不校验 d，对 d=0/d<0 静默接受。d=0 让请求永久
// 挂起，d<0 是非法值（Go time.Duration 负数无意义但合法）。
func TestWithTimeout_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c, _ := New(
		WithTimeout(15*time.Second),
		WithLogger(logger),
	)
	if c.http.Timeout != 15*time.Second {
		t.Fatalf("初始 Timeout 应 = 15s，实际 %v", c.http.Timeout)
	}

	// 再次 WithTimeout(-1) 应被拒绝
	WithTimeout(-1 * time.Second)(c)
	if c.http.Timeout != 15*time.Second {
		t.Errorf("WithTimeout(-1s) 应被拒绝，实际 Timeout=%v", c.http.Timeout)
	}
	if !strings.Contains(logBuf.String(), "负数超时被拒绝") {
		t.Errorf("应 warn '负数超时被拒绝'，实际 log：%s", logBuf.String())
	}
}

// TestWithTimeout_ZeroWarns 验证 WithTimeout(0) 被拒绝（保留原值）+ warn 提示风险。
func TestWithTimeout_ZeroWarns(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 白盒构造：先放好 logger，再调 WithTimeout，确保 warn 走 logBuf
	c := &Client{
		http:   newHTTPClient(),
		logger: logger,
	}
	WithTimeout(0)(c)
	if c.http.Timeout != 0 {
		t.Errorf("WithTimeout(0) 应设置 Timeout=0（net/http 无超时），实际 %v", c.http.Timeout)
	}
	if !strings.Contains(logBuf.String(), "无超时") {
		t.Errorf("应 warn '无超时' 风险，实际 log：%s", logBuf.String())
	}
}

// TestWithTimeout_ZeroDoesNotOverwriteExisting 回归测试（F9）：
// WithTimeout(0) 在已设置正数超时时**不应**清零已有超时。
// 历史 bug：WithTimeout(0) 仅 warn 但仍执行 c.http.Timeout = 0，
// 静默破坏调用方已配置的 15s 超时为"无超时"——后续请求可能永久挂起。
// 修复后 d==0 必须阻断赋值（与 d<0 行为对齐：拒绝 + warn 保持原值）。
func TestWithTimeout_ZeroDoesNotOverwriteExisting(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		http:   newHTTPClient(),
		logger: logger,
	}
	// 先设 15s
	WithTimeout(15 * time.Second)(c)
	if c.http.Timeout != 15*time.Second {
		t.Fatalf("初始 Timeout 应 = 15s，实际 %v", c.http.Timeout)
	}

	// 再 WithTimeout(0) 不应清零
	WithTimeout(0)(c)
	if c.http.Timeout != 15*time.Second {
		t.Errorf("WithTimeout(0) 不应清零已有超时，实际 Timeout=%v", c.http.Timeout)
	}
	if !strings.Contains(logBuf.String(), "无超时") {
		t.Errorf("应 warn '无超时' 风险，实际 log：%s", logBuf.String())
	}
}

// TestWithTimeout_NilHTTPWarns 回归测试（F9）：
// WithTimeout 在 c.http == nil 时不应静默 return——至少 warn 让用户感知。
// 历史 bug：WithTimeout 在 c.http == nil 时静默 return，调用方无法
// 知道 timeout 未生效。WithHTTPClient(nil) 是触发 c.http == nil 的
// 唯一外部路径——属于误用但需要被看见而非吞掉。
func TestWithTimeout_NilHTTPWarns(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		http:   nil, // 模拟 WithHTTPClient(nil) 之后的状态
		logger: logger,
	}

	// 不应 panic，应至少输出 warn
	WithTimeout(5 * time.Second)(c)

	if c.http != nil {
		t.Errorf("c.http==nil 路径不应创建 http client")
	}
	if !strings.Contains(logBuf.String(), "c.http 为 nil") {
		t.Errorf("应 warn 'c.http 为 nil'，实际 log：%s", logBuf.String())
	}
}

// ─── with_token_late_binding_test.go: WithToken late binding ───

// TestWithToken_LateBinding 回归测试：WithToken + WithSSOBase 顺序敏感性 bug。
// 历史 bug：WithToken 立即调 syncCookieToken 写 cookie 到当时的 c.ssoBaseURL，
// 若用户按 New(WithToken(t), WithSSOBase(u)) 顺序调用，token 写到 default
// SSO host 而非用户指定的 host，业务请求 0 cookie → 空数据。
// 修复后：WithToken 仅存到 c.pendingToken，New() 跑完所有 Options 后
// 才统一 syncCookieToken（此时 c.http.Jar / c.ssoBaseURL / c.baseURL
// 都是最终值）。
func TestWithToken_LateBinding(t *testing.T) {
	// mock 目标平台：记录收到的所有 X-Auth-Token cookie
	var receivedAuthToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("X-Auth-Token"); err == nil {
			receivedAuthToken = c.Value
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 关键：WithToken 在 WithSSOBase 之前调用（之前会写错 host）
	c, _ := New(
		WithToken("jwt-late-bind"),
		WithSSOBase(srv.URL),
		WithBaseURL(srv.URL),
	)

	// 触发实际请求，jar 应自动注入 X-Auth-Token cookie
	resp, err := c.http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	drainAndClose(resp.Body)

	if receivedAuthToken != "jwt-late-bind" {
		t.Errorf("请求未携带 X-Auth-Token=jwt-late-bind（实际收到 %q），WithToken cookie 写到了错误的 host",
			receivedAuthToken)
	}
}

// ─── with_token_trim_test.go: WithToken trim 空白 ───

// TestWithToken_TrimsWhiteSpace 验证传入带空格 token 时 WithToken 存储的是修剪后的值。
// 历史 bug：WithToken 用 strings.TrimSpace(token) == "" 校验空字符串，
// 但存储时用原始值 c.pendingToken = token（含前/后空白）。
// 后续 New() 末尾 syncCookieToken 写入 cookie 时 value 含有空白，
// 导致服务端解析出带空格的畸形 token，鉴权失败。
// 修复后：c.pendingToken = strings.TrimSpace(token)，与校验逻辑对称。
func TestWithToken_TrimsWhiteSpace(t *testing.T) {
	c := &Client{
		pendingToken: "",
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}

	WithToken("  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc  ")(c)
	want := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc"
	if c.pendingToken != want {
		t.Errorf("WithToken 应存储修剪后的值，实际 %q", c.pendingToken)
	}
}

// ─── withcustomocr_guard_test.go (F2): WithCustomOCR nil 守卫 ───

// TestWithCustomOCR_NilRejected 回归测试（F2）：
// WithCustomOCR(nil) 必须被拒绝，warn 提醒，保持当前 ocr 识别器（防止
// nil 静默覆盖已注入的识别器，导致后续 Login 因 c.ocr==nil 而返回
// ErrOCRNotConfigured）。
// 设计一致：与 WithLogger(nil) / WithHTTPClient(nil) 的 nil 拒绝守卫对称。
func TestWithCustomOCR_NilRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	mock := &mockCaptchaRecognizer{}
	c := &Client{
		ocr:    mock,
		logger: logger,
	}

	// nil 应被拒绝
	WithCustomOCR(nil)(c)
	if c.ocr != mock {
		t.Errorf("WithCustomOCR(nil) 应被拒绝，保持原 ocr 实例，实际被替换为 %v", c.ocr)
	}
	if !strings.Contains(logBuf.String(), "nil") || !strings.Contains(logBuf.String(), "WithCustomOCR") {
		t.Errorf("应 warn 包含 'nil' 和 'WithCustomOCR'，实际 log：%s", logBuf.String())
	}
}

// ─── withlogger_guard_test.go (A1): WithLogger nil 守卫 ───

// TestWithLogger_NilRejected 回归测试（A1）：
// WithLogger(nil) 必须被拒绝，warn 提醒，保持当前 logger（防止 nil 覆盖后
// 后续 c.logger.Warn/Debug/Error 全部 nil pointer panic）。
// 设计一致：与 WithTimeout(D1) / WithHTTPClient(F8) 的 nil 拒绝守卫对称。
func TestWithLogger_NilRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	c := &Client{
		logger: logger,
	}

	WithLogger(nil)(c)
	if c.logger != logger {
		t.Errorf("WithLogger(nil) 应被拒绝，保持原 logger，实际被替换")
	}
	if !strings.Contains(logBuf.String(), "nil") || !strings.Contains(logBuf.String(), "WithLogger") {
		t.Errorf("应 warn 包含 'nil' 和 'WithLogger'，实际 log：%s", logBuf.String())
	}
}

// ─── with_session_backoff_test.go: WithSessionBackoff 守卫 ───

// TestWithSessionBackoff_PositiveAccepted 验证 WithSessionBackoff(d>0) 设置字段。
func TestWithSessionBackoff_PositiveAccepted(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger: logger,
		sm:     &sessionManager{},
	}

	if c.sm.backoff != 0 {
		t.Fatalf("初始 sm.backoff 应 = 0，实际 %v", c.sm.backoff)
	}

	WithSessionBackoff(10 * time.Second)(c)
	if c.sm.backoff != 10*time.Second {
		t.Errorf("WithSessionBackoff(10s) 应设置 sm.backoff = 10s，实际 %v", c.sm.backoff)
	}

	if logBuf.Len() > 0 {
		t.Errorf("WithSessionBackoff(d>0) 不应输出 warn，实际 log: %s", logBuf.String())
	}
}

func TestWithSessionBackoff_ZeroRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger: logger,
		sm:     &sessionManager{backoff: 15 * time.Second},
	}

	WithSessionBackoff(0)(c)
	if c.sm.backoff != 15*time.Second {
		t.Errorf("WithSessionBackoff(0) 应被拒绝保持原值 15s，实际 %v", c.sm.backoff)
	}
	if !strings.Contains(logBuf.String(), "0") {
		t.Errorf("应 warn 包含 '0'，实际 log: %s", logBuf.String())
	}
}

func TestWithSessionBackoff_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger: logger,
		sm:     &sessionManager{backoff: 20 * time.Second},
	}

	WithSessionBackoff(-1 * time.Second)(c)
	if c.sm.backoff != 20*time.Second {
		t.Errorf("WithSessionBackoff(-1s) 应被拒绝保持原值 20s，实际 %v", c.sm.backoff)
	}
	if !strings.Contains(logBuf.String(), "负数") && !strings.Contains(logBuf.String(), "负") {
		t.Errorf("应 warn 包含 '负'，实际 log: %s", logBuf.String())
	}
}

// TestActivateWithBackoffCheck_UsesConfiguredBackoff 验证 sm.Activate
// 实际消费 sm.backoff 字段——而非硬编码 5s 默认值。
func TestActivateWithBackoffCheck_UsesConfiguredBackoff(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	errSentinel := errors.New("simulated last activation failure")
	sm := &sessionManager{
		backoff:         1 * time.Hour,
		lastErr:         errSentinel,
		lastAttempt:     time.Now(),
		lastFailedToken: "test-token",
		mu:              sync.Mutex{},
	}

	activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
		return nil, errors.New("should not be called")
	}

	_, err := sm.Activate(context.Background(), "test-token", activateFn)

	if err == nil {
		t.Fatal("1 小时 backoff 窗口内同 token 应被抑制返回错误，实际 nil")
	}
	if !errors.Is(err, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff，err=%v", err)
	}

	_ = logger
	_ = logBuf
}
