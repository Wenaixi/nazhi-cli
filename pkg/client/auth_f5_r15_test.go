// auth_f5_r15_test.go
// F5 验证：Login 流程 GetSchoolID + OCR 通过 errgroup 并发执行。
//
// 串行基线（旧）：InitSession → GetSchoolID → OCR → validateCaptcha → login
// 并发优化（新）：InitSession → (GetSchoolID || OCR) → validateCaptcha → login
//
// 通过 mock server 记录请求顺序和耗时，验证总耗时约等于 max(tGetSchoolID, tOCR) 而非 sum。
package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// fakeOCRF5 是简单 mock OCR，保证 fetchCaptchaImage 不走 http。
type fakeOCRF5 struct{}

func (*fakeOCRF5) Recognize(_ []byte) (string, error) { return "ABCD", nil }
func (*fakeOCRF5) Close() error                       { return nil }

// TestLogin_ParallelF5_R15C 验证 F5：GetSchoolID 与 OCR 并发执行。
//
// 设计：两个 handler 各自 sleep 50ms，并发场景总耗时 ~50ms（max 而非 sum 100ms）。
func TestLogin_ParallelF5_R15C(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过耗时测试")
	}
	t.Parallel()

	var (
		mu       sync.Mutex
		events   []string
		startBar = make(chan struct{})
		gsReady  = make(chan struct{})
		ocrReady = make(chan struct{})
	)

	record := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	mux := http.NewServeMux()

	// InitSession
	mux.HandleFunc("/uiStudentLogin/login", func(w http.ResponseWriter, r *http.Request) {
		record("init")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{}`)
	})

	// GetSchoolID
	mux.HandleFunc("/teacher/auth/studentLogin/getSchoolIdByStudentNumber", func(w http.ResponseWriter, r *http.Request) {
		record("gs-start")
		close(gsReady)
		<-ocrReady
		<-startBar
		time.Sleep(50 * time.Millisecond)
		record("gs-end")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"code":1,"dataList":[{"school_id":"123","NAME":"测试学校"}]}`)
	})

	// OCR 验证码图片
	mux.HandleFunc("/kaptcha/kaptcha.jpg", func(w http.ResponseWriter, r *http.Request) {
		record("ocr-start")
		close(ocrReady)
		<-gsReady
		<-startBar
		time.Sleep(50 * time.Millisecond)
		record("ocr-end")
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte{0xFF, 0xD8, 0xFF})
	})

	// validateCaptcha
	mux.HandleFunc("/uiStudentLogin/validateCaptcha", func(w http.ResponseWriter, r *http.Request) {
		record("captcha")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"code":1}`)
	})

	// Login 提交
	mux.HandleFunc("/teacher/auth/studentLogin/validate", func(w http.ResponseWriter, r *http.Request) {
		record("login")
		w.WriteHeader(http.StatusOK)
		raw := json.RawMessage(`{"token":"test-jwt","expires_in":86400}`)
		resp, _ := json.Marshal(types.UnifiedResponse{
			Code:       1,
			ReturnData: &raw,
		})
		w.Write(resp)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := New(
		WithSSOBase(srv.URL),
		WithBaseURL(srv.URL),
		WithCustomOCR(&fakeOCRF5{}),
		WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	// 两个 goroutine 都到 barrier 后放行
	go func() {
		<-gsReady
		<-ocrReady
		close(startBar)
	}()

	start := time.Now()
	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "testuser",
		Password: "testpass",
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if resp == nil || resp.Token != "test-jwt" {
		t.Fatalf("Login 结果异常: %+v", resp)
	}

	mu.Lock()
	t.Logf("事件顺序: %v", events)
	mu.Unlock()
	t.Logf("总耗时: %v", elapsed)

	if elapsed > 150*time.Millisecond {
		t.Logf("WARNING: 耗时 %v 略高（期望约 80ms），可能并发未完全生效", elapsed)
	} else {
		t.Logf("F5 验证通过：耗时 ~%v（串行基线 ~100ms）证明并发生效", elapsed)
	}
}
