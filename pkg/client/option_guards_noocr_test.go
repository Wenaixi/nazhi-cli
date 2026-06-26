//go:build !ddddocr
// +build !ddddocr

// Package client 验证 !ddddocr 构建下 WithOCRConcurrency 占位行为。
//
// ddddocr 未启用（Nazhi-auto CGO_ENABLED=0 场景）时：
//   - WithOCRConcurrency 必须是 no-op + warn，不能 panic 或默默替换 c.ocr
//   - c.ocr 保持调用方注入的 WithCustomOCR 实例不动
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

// TestWithOCRConcurrency_NoDdddOCR_Build 验证 !ddddocr 构建下占位实现：
//   - 调用 WithOCRConcurrency 不会替换 c.ocr（保持 WithCustomOCR 注入）
//   - 必输出 warn 提示 ddddocr 未启用，引导调用方改用 WithCustomOCR
func TestWithOCRConcurrency_NoDdddOCR_Build(t *testing.T) {
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
