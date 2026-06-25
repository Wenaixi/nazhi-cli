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
//
// 设计：
//   - mock server 在步骤 4（getMyInfo）返回 500
//   - 10 个 goroutine 同时调用 activateSessionIfNeeded
//   - 无 backoff 时：10 个 goroutine 依次串行执行 10 次完整 4 步激活 → 步骤 4 被调用 10 次
//   - 有 backoff 时：第 1 个激活失败后缓存错误，后续 9 个直接返回 → 步骤 4 被调用 1 次
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
	// 设置足够大的 backoff 保证所有 goroutine 在窗口内返回缓存错误
	c.sessionBackoff = time.Hour

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	var errCount int32
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if err := c.activateSessionIfNeeded(context.Background(), "shared-token"); err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	// 验证：所有 goroutine 都返回错误（至少不 panic 或死锁）
	if n := atomic.LoadInt32(&errCount); n != goroutines {
		t.Errorf("期望 %d 个 goroutine 返回错误，实际 %d", goroutines, n)
	}
	// 验证：步骤 4 仅被执行 1 次（非 10 次），thundering herd 被抑制
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4（getMyInfo）期望 1 次 HTTP 请求，实际 %d — thundering herd 未被抑制", got)
	}
}
