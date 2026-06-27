package client

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestWithCustomOCR_NilRejected 回归测试（F2）：
// WithCustomOCR(nil) 必须被拒绝，warn 提醒，保持当前 ocr 识别器（防止
// nil 静默覆盖已注入的识别器，导致后续 Login 因 c.ocr==nil 而返回
// ErrOCRNotConfigured）。
//
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
