//go:build !ddddocr
// +build !ddddocr

// Package client 验证 !ddddocr 构建下 WithOCRConcurrency 占位行为。
// ddddocr 未启用（Nazhi-auto CGO_ENABLED=0 场景）时：
// - WithOCRConcurrency 必须是 no-op + warn，不能 panic 或默默替换 c.ocr
// - c.ocr 保持调用方注入的 WithCustomOCR 实例不动
package client

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// mockCaptchaRecognizer 与 option_guards_test.go 同款（占位实现版本）：
// 两个 build-tag 文件分别编译，同 build 下只有一个被包含，类型名故意
// 保持一致以便对照阅读——不要改成不同名。
type mockCaptchaRecognizer struct {
	closed bool
}

func (m *mockCaptchaRecognizer) Recognize([]byte) (string, error) { return "ok", nil }
func (m *mockCaptchaRecognizer) Close() error                     { m.closed = true; return nil }

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
