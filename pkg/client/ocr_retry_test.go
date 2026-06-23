package client

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/internal/version"
)

// newClientForOCRTest 是简化版测试 helper（package client 内部可见版本）。
func newClientForOCRTest(ssoURL string, ocr captchaRecognizer) *Client {
	return &Client{
		ssoBaseURL: ssoURL,
		baseURL:    ssoURL,
		uploadURL:  ssoURL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        ocr,
	}
}

// ─── 计数 mock OCR ───

// countMockOCR 跟踪调用次数，按"先失败 N 次再成功"模式工作。
// 模拟"同一张图 OCR N 次都失败"的场景——配合多图多试策略触发换图。
type countMockOCR struct {
	failBeforeSuccess int32 // 前 N 次返回 error（模拟 OCR 置信度低）
	recognizeCalls    int32 // 总调用次数
	returnText        string
}

func (m *countMockOCR) Recognize(_ []byte) (string, error) {
	n := atomic.AddInt32(&m.recognizeCalls, 1)
	if n <= atomic.LoadInt32(&m.failBeforeSuccess) {
		return "", errOCRMockFailed
	}
	return m.returnText, nil
}

// errOCRMockFailed 是 mock 专用错误，区别于真实 OCR 错误。
var errOCRMockFailed = &mockOCRErr{msg: "mock OCR 模拟识别失败"}

// mockOCRErr 是 mock 错误类型。
type mockOCRErr struct{ msg string }

func (e *mockOCRErr) Error() string { return e.msg }

// ─── 测试：多图多试策略 ───

// TestOCRRetry_SucceedsOnFirstImage 验证：1 张图第 1 次就成功。
// 期望：1 次图片获取 + 1 次 OCR 调用。
func TestOCRRetry_SucceedsOnFirstImage(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	mock := &countMockOCR{failBeforeSuccess: 0, returnText: "ab12"}
	c := newClientForOCRTest(sso.URL, mock)
	c.ocr = mock

	got, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if got != "ab12" {
		t.Fatalf("expected 'ab12', got %q", got)
	}
	if got := atomic.LoadInt32(&imageFetches); got != 1 {
		t.Errorf("expected 1 image fetch, got %d", got)
	}
	if got := atomic.LoadInt32(&mock.recognizeCalls); got != 1 {
		t.Errorf("expected 1 OCR call, got %d", got)
	}
}

// TestOCRRetry_FailsAcrossImages 验证跨图重试：每张图 OCR 1 次后换图
// （ddddocr 确定性下同图重试无意义，1×99 策略）。
// 期望：3 次失败后第 4 张图成功 = 4 imageFetches + 4 recognizeCalls。
func TestOCRRetry_FailsAcrossImages(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	// 前 3 次 fail，第 4 次成功（=第 4 张图第 1 次尝试）
	mock := &countMockOCR{failBeforeSuccess: 3, returnText: "xy34"}
	c := newClientForOCRTest(sso.URL, mock)
	c.ocr = mock

	got, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if got != "xy34" {
		t.Fatalf("expected 'xy34', got %q", got)
	}
	if got := atomic.LoadInt32(&imageFetches); got != 4 {
		t.Errorf("expected 4 image fetches (3 张图各 1 次失败 + 第 4 张成功), got %d", got)
	}
	if got := atomic.LoadInt32(&mock.recognizeCalls); got != 4 {
		t.Errorf("expected 4 OCR calls (3 fail + 1 success), got %d", got)
	}
}

// TestOCRRetry_Fails2ThenSucceedsAfter2Images 验证：前 2 张图各 1 次失败，
// 第 3 张图第 1 次就成功（1×99 策略下"attempt 3"语义变为"第 3 张图"）。
// 期望：3 imageFetches + 3 recognizeCalls。
func TestOCRRetry_Fails2ThenSucceedsAfter2Images(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	mock := &countMockOCR{failBeforeSuccess: 2, returnText: "ok99"}
	c := newClientForOCRTest(sso.URL, mock)
	c.ocr = mock

	got, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if got != "ok99" {
		t.Fatalf("expected 'ok99', got %q", got)
	}
	if got := atomic.LoadInt32(&imageFetches); got != 3 {
		t.Errorf("expected 3 image fetches (2 fail + 1 success), got %d", got)
	}
	if got := atomic.LoadInt32(&mock.recognizeCalls); got != 3 {
		t.Errorf("expected 3 OCR calls (2 fail + 1 success), got %d", got)
	}
}

// TestOCRRetry_AllImagesFail 验证最坏情况：11 张图全 9 次都失败。
// 期望：11 次图片获取 + 99 次 OCR 调用 + 错误信息。
func TestOCRRetry_AllImagesFail(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	// 全部失败（远超 99 次）
	mock := &countMockOCR{failBeforeSuccess: 9999, returnText: "never"}
	c := newClientForOCRTest(sso.URL, mock)
	c.ocr = mock

	got, err := c.ocrRecognizeWithRetry(context.Background())
	if err == nil {
		t.Fatalf("expected error after all retries, got text %q", got)
	}
	if got != "" {
		t.Errorf("expected empty text on failure, got %q", got)
	}
	if got := atomic.LoadInt32(&imageFetches); got != int32(maxOCRImagesTotal) {
		t.Errorf("expected %d image fetches, got %d", maxOCRImagesTotal, got)
	}
	if got := atomic.LoadInt32(&mock.recognizeCalls); got != int32(maxOCRImagesTotal*maxOCRAttemptsPerImage) {
		t.Errorf("expected %d OCR calls (11×9=99), got %d",
			maxOCRImagesTotal*maxOCRAttemptsPerImage, got)
	}
}

// TestOCRRetry_BlankResultRetried 验证：OCR 返回空字符串（非错误）也算失败，重试。
// 模拟"图片能识别但内容为空"场景。
func TestOCRRetry_BlankResultRetried(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	// blankMockOCR：前 5 次返回空字符串，第 6 次成功
	blankMock := &blankThenSuccessMock{blankBefore: 5, returnText: "good"}
	c := newClientForOCRTest(sso.URL, blankMock)
	c.ocr = blankMock

	got, err := c.ocrRecognizeWithRetry(context.Background())
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if got != "good" {
		t.Fatalf("expected 'good', got %q", got)
	}
	if calls := atomic.LoadInt32(&blankMock.calls); calls != 6 {
		t.Errorf("expected 6 calls (5 blank + 1 success), got %d", calls)
	}
}

// blankThenSuccessMock 前 N 次返回空字符串，之后返回成功。
type blankThenSuccessMock struct {
	blankBefore int32
	calls       int32
	returnText  string
}

func (m *blankThenSuccessMock) Recognize(_ []byte) (string, error) {
	n := atomic.AddInt32(&m.calls, 1)
	if n <= atomic.LoadInt32(&m.blankBefore) {
		return "", nil // 空字符串，无 error
	}
	return m.returnText, nil
}

// TestOCRRetry_ImageFetchFails 验证：图片获取失败也换图。
// 期望：尝试 11 次图片获取都失败。
func TestOCRRetry_ImageFetchFails(t *testing.T) {
	var imageFetches int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kaptcha/kaptcha.jpg" {
			atomic.AddInt32(&imageFetches, 1)
			http.Error(w, "captcha service down", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer sso.Close()

	mock := &countMockOCR{failBeforeSuccess: 0, returnText: "unused"}
	c := newClientForOCRTest(sso.URL, mock)
	c.ocr = mock

	_, err := c.ocrRecognizeWithRetry(context.Background())
	if err == nil {
		t.Fatalf("expected error when all image fetches fail")
	}
	if got := atomic.LoadInt32(&imageFetches); got != int32(maxOCRImagesTotal) {
		t.Errorf("expected %d image fetch attempts, got %d", maxOCRImagesTotal, got)
	}
	if got := atomic.LoadInt32(&mock.recognizeCalls); got != 0 {
		t.Errorf("expected 0 OCR calls (no image ever succeeded), got %d", got)
	}
}

// TestOCRRetry_Constants 兜底测试：常量值符合预期（1 × 99 = 99）。
// ddddocr 确定性下同图重试无意义，把所有重试预算分配给换图。
func TestOCRRetry_Constants(t *testing.T) {
	if maxOCRAttemptsPerImage != 1 {
		t.Errorf("maxOCRAttemptsPerImage = %d, want 1", maxOCRAttemptsPerImage)
	}
	if maxOCRImagesTotal != 99 {
		t.Errorf("maxOCRImagesTotal = %d, want 99", maxOCRImagesTotal)
	}
	if maxOCRAttemptsPerImage*maxOCRImagesTotal != 99 {
		t.Errorf("1 × 99 should equal 99, got %d",
			maxOCRAttemptsPerImage*maxOCRImagesTotal)
	}
	t.Logf("nazhi %s — OCR 重试策略: %d 张图 × %d 次 = %d 次总尝试上限（ddddocr 确定性下把预算全放换图）",
		version.Version, maxOCRImagesTotal, maxOCRAttemptsPerImage,
		maxOCRImagesTotal*maxOCRAttemptsPerImage)
}
