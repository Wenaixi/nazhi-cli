// Package client 白盒测试：OCR Recognize panic recover。
package client

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// panicMockOCR 在 Recognize 中 panic，模拟 CGO 崩溃 / mock 误实现。
type panicMockOCR struct{}

func (m *panicMockOCR) Recognize([]byte) (string, error) {
	panic("mock OCR intentional panic")
}
func (m *panicMockOCR) Close() error { return nil }

// TestOCRRecognize_PanicRecover 回归测试（A5）：
// Recognize panic 应被 recover 并转为 error，不崩溃进程。
func TestOCRRecognize_PanicRecover(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("safeOCRRecognize 应 recover panic，但 panic 泄漏: %v", r)
		}
	}()

	c := &Client{}
	// 直接设置 ocr 字段（跳过 New，避免 build tag 干扰）
	c.ocr = &panicMockOCR{}

	text, err := c.safeOCRRecognize([]byte("fake-img"))
	if err == nil {
		t.Error("safeOCRRecognize 在 mock panic 时应返回 error，实际 nil")
	}
	if text != "" {
		t.Errorf("safeOCRRecognize 在 mock panic 时 text 应为空，实际 %q", text)
	}
	// 验证 error 包含 panic 原因
	if !errors.Is(err, ErrOCRPanic) {
		t.Errorf("safeOCRRecognize 在 mock panic 时应返回 %v，实际 %v", ErrOCRPanic, err)
	}
}

// TestOCRRecognize_PanicNil 验证 ocr 为 nil 时 safeOCRRecognize 应返回 error。
func TestOCRRecognize_PanicNil(t *testing.T) {
	c := &Client{}
	// ocr 为 nil 时 safeOCRRecognize 应直接返回 error
	text, err := c.safeOCRRecognize([]byte("fake"))
	if err == nil {
		t.Error("ocr == nil 时应返回 error，实际 nil")
	}
	if text != "" {
		t.Errorf("ocr == nil 时 text 应为空，实际 %q", text)
	}
}

// TestOCRRecognize_PanicRecoverInRetry 验证 ocrRecognizeWithRetry 中 panic 也被 recover。
func TestOCRRecognize_PanicRecoverInRetry(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/kaptcha/kaptcha.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg"))
	})
	sso := httptest.NewServer(mux)
	defer sso.Close()

	c := &Client{
		ssoBaseURL: sso.URL,
		baseURL:    sso.URL,
		uploadURL:  sso.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
	}
	c.ocr = &panicMockOCR{}

	text, err := c.ocrRecognizeWithRetry(context.Background())
	if err == nil {
		t.Error("ocrRecognizeWithRetry 在 mock panic 时应返回 error，实际 nil")
	}
	_ = text
}
