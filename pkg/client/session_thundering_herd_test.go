package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestActivateSessionIfNeeded_ThunderingHerd 回归测试 S1：
// 10 个 goroutine 并发 activateSessionIfNeeded，第 1 个失败后其余 9 个
// 应直接返回缓存错误，不重复执行 4 步激活（thundering herd 抑制）。
func TestActivateSessionIfNeeded_ThunderingHerd(t *testing.T) {
	var step4Count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)
	c.sm.backoff = time.Hour

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	var errCount int32
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.activateSessionIfNeeded(context.Background(), "shared-token"); err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if n := atomic.LoadInt32(&errCount); n != goroutines {
		t.Errorf("期望 %d 个 goroutine 返回错误，实际 %d", goroutines, n)
	}
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4 期望 1 次 HTTP 请求，实际 %d — thundering herd 未被抑制", got)
	}
}
