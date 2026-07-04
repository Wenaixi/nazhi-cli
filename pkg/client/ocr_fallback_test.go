// Package client 白盒测试：OCR 双策略降级（primary + fallback）。
package client

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ─── 特殊 mock ───

// failAfterCountOCR 前 N 次成功，之后全部失败。
// 用于测试"primary 能搞定时 fallback 不触发"的场景。
type failAfterCountOCR struct {
	successBefore int32 // 前 N 次识别成功
	calls         int32
	returnText    string
}

func (m *failAfterCountOCR) Recognize(_ []byte) (string, error) {
	n := atomic.AddInt32(&m.calls, 1)
	if n <= atomic.LoadInt32(&m.successBefore) {
		return m.returnText, nil
	}
	return "", errOCRMockFailed
}
func (m *failAfterCountOCR) Close() error { return nil }

// ─── 测试用例 ───

// TestOCRFallback_PrimarySucceeds_NoFallback 验证：
// primary 成功时，fallback 完全不触发，返回 primary 结果。
func TestOCRFallback_PrimarySucceeds_NoFallback(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		if r.URL.Path == "/uiStudentLogin/validateCaptcha" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	primary := &countMockOCR{failBeforeSuccess: 0, returnText: "ok12"}
	fallback := &countMockOCR{failBeforeSuccess: 0, returnText: "fallback"}
	_ = fallback // 确认不会被调用

	c := &Client{
		ssoBaseURL:    sso.URL,
		baseURL:       sso.URL,
		uploadURL:     sso.URL,
		http:          newHTTPClient(),
		logger:        slog.New(slog.DiscardHandler),
		ocr:           primary,
		fallbackOCR:   fallback,
	}

	got, fbUsed, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("期望成功，收到错误: %v", err)
	}
	if got != "ok12" {
		t.Errorf("期望 'ok12'，实际 %q", got)
	}
	if fbUsed {
		t.Error("primary 成功时不应使用 fallback")
	}
	// fallback 不应被调用
	if calls := atomic.LoadInt32(&fallback.recognizeCalls); calls != 0 {
		t.Errorf("fallback 不应被调用（被调 %d 次）", calls)
	}
}

// TestOCRFallback_PrimaryFails_FallbackSucceeds 验证：
// primary 9 张全失败 → 自动降级 fallback → fallback 成功。
func TestOCRFallback_PrimaryFails_FallbackSucceeds(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		if r.URL.Path == "/uiStudentLogin/validateCaptcha" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	// primary 全部失败（>9 次）
	primary := &countMockOCR{failBeforeSuccess: 9999, returnText: "never"}
	// fallback 第 1 次就成功
	fallback := &countMockOCR{failBeforeSuccess: 0, returnText: "fb99"}

	c := &Client{
		ssoBaseURL:    sso.URL,
		baseURL:       sso.URL,
		uploadURL:     sso.URL,
		http:          newHTTPClient(),
		logger:        slog.New(slog.DiscardHandler),
		ocr:           primary,
		fallbackOCR:   fallback,
	}

	// 缩短超时，加速测试
	origTimeout := ocrTimeout
	ocrTimeout = 5 * time.Second
	defer func() { ocrTimeout = origTimeout }()

	got, fbUsed, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("期望 fallback 成功，收到错误: %v", err)
	}
	if got != "fb99" {
		t.Errorf("期望 fallback 结果 'fb99'，实际 %q", got)
	}
	if !fbUsed {
		t.Error("primary 失败时应该使用 fallback")
	}
	// primary 应该被调用过（至少 maxOCRImagesTotal 次）
	if calls := atomic.LoadInt32(&primary.recognizeCalls); calls < int32(maxOCRImagesTotal) {
		t.Errorf("primary 应被调至少 %d 次，实际 %d", maxOCRImagesTotal, calls)
	}
	// fallback 应该被调用过
	if calls := atomic.LoadInt32(&fallback.recognizeCalls); calls == 0 {
		t.Error("fallback 应该被调用，实际为 0")
	}
}

// TestOCRFallback_BothFail 验证：
// primary 和 fallback 都全部失败 → 返回错误。
func TestOCRFallback_BothFail(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		if r.URL.Path == "/uiStudentLogin/validateCaptcha" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	primary := &countMockOCR{failBeforeSuccess: 9999, returnText: "never"}
	fallback := &countMockOCR{failBeforeSuccess: 9999, returnText: "never"}

	c := &Client{
		ssoBaseURL:    sso.URL,
		baseURL:       sso.URL,
		uploadURL:     sso.URL,
		http:          newHTTPClient(),
		logger:        slog.New(slog.DiscardHandler),
		ocr:           primary,
		fallbackOCR:   fallback,
	}

	origTimeout := ocrTimeout
	ocrTimeout = 5 * time.Second
	defer func() { ocrTimeout = origTimeout }()

	got, fbUsed, err := c.ocrRecognizeWithRetry(context.Background())
	if err == nil {
		t.Fatalf("期望错误，实际成功 text=%q", got)
	}
	if fbUsed {
		t.Error("两者都失败时 fallbackUsed 应为 false")
	}
	if got != "" {
		t.Errorf("失败时 text 应为空，实际 %q", got)
	}
}

// TestOCRFallback_NoFallbackConfigured 验证：
// fallbackOCR 为 nil 时，primary 失败后直接返回错误，不降级。
func TestOCRFallback_NoFallbackConfigured(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		if r.URL.Path == "/uiStudentLogin/validateCaptcha" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	primary := &countMockOCR{failBeforeSuccess: 9999, returnText: "never"}

	c := &Client{
		ssoBaseURL:  sso.URL,
		baseURL:     sso.URL,
		uploadURL:   sso.URL,
		http:        newHTTPClient(),
		logger:      slog.New(slog.DiscardHandler),
		ocr:         primary,
		fallbackOCR: nil, // 未配置 fallback
	}

	origTimeout := ocrTimeout
	ocrTimeout = 5 * time.Second
	defer func() { ocrTimeout = origTimeout }()

	got, fbUsed, err := c.ocrRecognizeWithRetry(context.Background())
	if err == nil {
		t.Fatalf("期望错误（无 fallback），实际成功 text=%q", got)
	}
	if fbUsed {
		t.Error("fallback 未配置时 fallbackUsed 应为 false")
	}
	if got != "" {
		t.Errorf("失败时 text 应为空，实际 %q", got)
	}
}

// TestOCRFallback_PrimaryFails3_FallbackSucceeds 验证：
// primary 前 3 张图失败，fallback 第 1 张就成功。
// 验证 fallback 重新拉新图（不是用 primary 的老图）。
func TestOCRFallback_PrimaryFails3_FallbackSucceeds(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		if r.URL.Path == "/uiStudentLogin/validateCaptcha" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	primary := &countMockOCR{failBeforeSuccess: 9999, returnText: "never"}
	fallback := &countMockOCR{failBeforeSuccess: 0, returnText: "fbOK"}

	c := &Client{
		ssoBaseURL:    sso.URL,
		baseURL:       sso.URL,
		uploadURL:     sso.URL,
		http:          newHTTPClient(),
		logger:        slog.New(slog.DiscardHandler),
		ocr:           primary,
		fallbackOCR:   fallback,
	}

	origTimeout := ocrTimeout
	ocrTimeout = 10 * time.Second
	defer func() { ocrTimeout = origTimeout }()

	got, fbUsed, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("期望 fallback 成功，收到错误: %v", err)
	}
	if got != "fbOK" {
		t.Errorf("期望 'fbOK'，实际 %q", got)
	}
	if !fbUsed {
		t.Error("primary 失败时应该使用 fallback")
	}

	// 验证 fallback 使用了新图片（total fetches > maxOCRImagesTotal）
	totalFetches := atomic.LoadInt32(&imageFetches)
	if totalFetches <= int32(maxOCRImagesTotal) {
		t.Errorf("fallback 后总 fetches 应 > %d（表示换了新图），实际 %d", maxOCRImagesTotal, totalFetches)
	}
}
