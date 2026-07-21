//go:build ddddocr
// +build ddddocr

// option_guards_ddddocr_test.go (ddddocr 构建) 聚合 ddddocr 构建下
// 选项守卫 + OCR 并发调度的白盒测试。
// !ddddocr 构建见 option_test.go（占位实现）。
package client

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// mockCaptchaRecognizer 测试用 mock：只记录被设置过、Close 不报错。
type mockCaptchaRecognizer struct {
	closed bool
}

func (m *mockCaptchaRecognizer) Recognize([]byte) (string, error) { return "ok", nil }
func (m *mockCaptchaRecognizer) Close() error                     { m.closed = true; return nil }

// ─── option_guards_test.go (ddddocr 构建): 6 个 Option 守卫 ───

// TestWithSSOBase_EmptyRejected 回归测试（D1）：
// WithSSOBase("") 必须被拒绝，保持当前 ssoBaseURL 值（防止用空 URL
// 覆盖掉 New() 阶段已设的 defaultSSOBase，破坏下游所有 SSO 调用）。
// 历史 bug：6 个 URL/资源型 Option（WithSSOBase / WithBaseURL / WithUploadURL /
// WithHTTPClient / WithOCRConcurrency / WithToken）均无任何校验，与 F9 给
// WithTimeout 加的 nil/0/负数三重 warn+拒绝守卫不对称——空字符串会静默覆盖
// defaultSSOBase 为空，导致后续 ssoURL(...) 拼接出 "https://" 之类畸形 URL。
// 修复后：空字符串必须 warn + 不修改字段（与 WithTimeout d=0 行为对齐）。
func TestWithSSOBase_EmptyRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 白盒构造：先放好 logger，再调 WithSSOBase，确保 warn 走 logBuf
	c := &Client{
		ssoBaseURL: defaultSSOBase, // 模拟 New() 已设的默认值
		logger:     logger,
	}

	// 空字符串应被拒绝
	WithSSOBase("")(c)
	if c.ssoBaseURL != defaultSSOBase {
		t.Errorf("WithSSOBase(\"\") 应被拒绝，保持原值 %q，实际 %q",
			defaultSSOBase, c.ssoBaseURL)
	}
	if !strings.Contains(logBuf.String(), "空") || !strings.Contains(logBuf.String(), "WithSSOBase") {
		t.Errorf("应 warn 包含 '空' 和 'WithSSOBase'，实际 log：%s", logBuf.String())
	}
}

// TestWithBaseURL_EmptyRejected 回归测试（D1）：WithBaseURL("") 必须被拒绝，
// 保持当前 baseURL 值（防止空字符串覆盖 defaultBaseURL 导致 bizURL 拼出畸形 URL）。
func TestWithBaseURL_EmptyRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		baseURL: defaultBaseURL,
		logger:  logger,
	}

	WithBaseURL("")(c)
	if c.baseURL != defaultBaseURL {
		t.Errorf("WithBaseURL(\"\") 应被拒绝，保持原值 %q，实际 %q",
			defaultBaseURL, c.baseURL)
	}
	if !strings.Contains(logBuf.String(), "空") || !strings.Contains(logBuf.String(), "WithBaseURL") {
		t.Errorf("应 warn 包含 '空' 和 'WithBaseURL'，实际 log：%s", logBuf.String())
	}
}

// TestWithUploadURL_EmptyRejected 回归测试（D1）：WithUploadURL("") 必须被拒绝，
// 保持当前 uploadURL 值（防止空字符串覆盖 defaultUploadURL 导致 uploadServiceURL 拼出畸形 URL）。
func TestWithUploadURL_EmptyRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		uploadURL: defaultUploadURL,
		logger:    logger,
	}

	WithUploadURL("")(c)
	if c.uploadURL != defaultUploadURL {
		t.Errorf("WithUploadURL(\"\") 应被拒绝，保持原值 %q，实际 %q",
			defaultUploadURL, c.uploadURL)
	}
	if !strings.Contains(logBuf.String(), "空") || !strings.Contains(logBuf.String(), "WithUploadURL") {
		t.Errorf("应 warn 包含 '空' 和 'WithUploadURL'，实际 log：%s", logBuf.String())
	}
}

// TestWithHTTPClient_NilRejected 回归测试（D1）：WithHTTPClient(nil) 必须被拒绝，
// 保持当前 http 客户端（防止 nil 静默覆盖默认带 cookie jar 的客户端，
// 导致后续请求 0 cookie → 空 dataList）。
func TestWithHTTPClient_NilRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	defaultHTTP := newHTTPClient()
	c := &Client{
		http:   defaultHTTP,
		logger: logger,
	}

	// nil 应被拒绝
	WithHTTPClient(nil)(c)
	if c.http != defaultHTTP {
		t.Errorf("WithHTTPClient(nil) 应被拒绝，保持原 http client，实际被替换为 %v", c.http)
	}
	if !strings.Contains(logBuf.String(), "nil") || !strings.Contains(logBuf.String(), "WithHTTPClient") {
		t.Errorf("应 warn 包含 'nil' 和 'WithHTTPClient'，实际 log：%s", logBuf.String())
	}
}

// TestWithOCRConcurrency_NegativeRejected 回归测试（D1）：
// WithOCRConcurrency(-1) 必须被拒绝，warn 提醒，保持当前 ocr 识别器（防止
// 负数被静默截 0 后用默认值覆盖调用方已注入的自定义识别器）。
// 历史 bug：WithOCRConcurrency 对 n<0 仅 `if n<0 { n=0 }` 静默截 0，
// 然后无脑 c.ocr = ocr.NewPool(0) 覆盖——若调用方先用 WithCustomOCR 注入
// mock，WithOCRConcurrency(-1) 会静默清掉 mock，导致后续 Login 走默认 OCR。
// !ddddocr 构建下此测试移到 option_guards_noocr_test.go（占位实现行为）。
func TestWithOCRConcurrency_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mock := &mockCaptchaRecognizer{}
	c := &Client{
		ocr:    mock, // 模拟 WithCustomOCR(mock) 已注入
		logger: logger,
	}

	// 负数应被拒绝
	WithOCRConcurrency(-1)(c)
	if c.ocr != mock {
		t.Errorf("WithOCRConcurrency(-1) 应被拒绝，保持原 ocr 实例，实际被替换")
	}
	if !strings.Contains(logBuf.String(), "负") || !strings.Contains(logBuf.String(), "WithOCRConcurrency") {
		t.Errorf("应 warn 包含 '负' 和 'WithOCRConcurrency'，实际 log：%s", logBuf.String())
	}
}

// TestWithOCRConcurrency_DoesNotOverwriteCustomOCR 回归测试（group-H）：
// WithCustomOCR(mock) 之后再 WithOCRConcurrency(n>=0) 不得用 ocr.NewPool 覆盖 mock。
// 历史 bug：ddddocr 构建下 n>=0 无条件 c.ocr = ocr.NewPool(n)，导致
// New(WithCustomOCR(mock), WithOCRConcurrency(1)) 静默丢掉 mock，Login 走真实 OCR。
func TestWithOCRConcurrency_DoesNotOverwriteCustomOCR(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	mock := &mockCaptchaRecognizer{}
	c := &Client{
		ocr:    mock, // 模拟 WithCustomOCR(mock) 已注入
		logger: logger,
	}

	// n=0 / n=1 / n>1 均不得覆盖自定义识别器
	for _, n := range []int{0, 1, 2} {
		WithOCRConcurrency(n)(c)
		if c.ocr != mock {
			t.Fatalf("WithOCRConcurrency(%d) 不得覆盖 WithCustomOCR 注入的 mock，实际被替换", n)
		}
	}
}

// TestWithOCRConcurrency_StillReplacesDefaultPool 保证修复后默认 ddddocr Pool
// 仍可被 WithOCRConcurrency 替换（仅保护自定义识别器，不误伤内置 Pool）。
func TestWithOCRConcurrency_StillReplacesDefaultPool(t *testing.T) {
	old := defaultOCR()
	if old == nil {
		t.Fatal("ddddocr 构建下 defaultOCR() 不应为 nil")
	}
	c := &Client{
		ocr:    old,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	WithOCRConcurrency(2)(c)
	if c.ocr == old {
		t.Error("WithOCRConcurrency 应对内置 *ocr.Pool 生效（创建新 Pool），不应保持原实例")
	}
	if c.ocr == nil {
		t.Fatal("WithOCRConcurrency 替换后 c.ocr 不应为 nil")
	}
}

// TestWithToken_EmptyOrWhitespaceRejected 回归测试（D1）：
// WithToken("") / WithToken(" ") / WithToken("\t\n") 必须被拒绝，
// 保持当前 pendingToken 值（防止空/纯空白 token 静默覆盖有效 token，
// 后续 New() 末尾 syncCookieToken 写入空 cookie 导致业务鉴权失败）。
func TestWithToken_EmptyOrWhitespaceRejected(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"spaces", "   "},
		{"tabs_and_newlines", "\t\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			c := &Client{
				pendingToken: "valid-prev-token",
				logger:       logger,
			}

			WithToken(tt.token)(c)
			if c.pendingToken != "valid-prev-token" {
				t.Errorf("WithToken(%q) 应被拒绝，保持原 pendingToken，实际 %q",
					tt.token, c.pendingToken)
			}
			if !strings.Contains(logBuf.String(), "空") && !strings.Contains(logBuf.String(), "空白") {
				t.Errorf("应 warn 包含 '空' 或 '空白'，实际 log：%s", logBuf.String())
			}
			if !strings.Contains(logBuf.String(), "WithToken") {
				t.Errorf("应 warn 包含 'WithToken'，实际 log：%s", logBuf.String())
			}
		})
	}
}
