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

// TestActivateSessionIfNeeded_BackoffHundredConcurrent 验证 100 个 goroutine
// 同时调用 activateSessionIfNeeded（同 token 激活失败）时 backoff 缓存生效：
//  1. 步骤 4 只会被执行 1 次（首 goroutine 触发完整的 4 步）
//  2. 其余 99 个 goroutine 在 backoff 窗口内命中 lastActivationErr 缓存
//  3. 无 data race（-race 检测器干净）
//
// 设计动机：backoff 缓存虽然在 CLI 单进程场景无 hit（CLI 不会先失败再重试），
// 但在 SDK 多 goroutine 场景（如 100 路并发 FetchTasks）中，
// backoff 可有效抑制 thundering herd——首路 4 步失败后其余 99 路直接返回，
// 避免 99 次重复的 4 步激增加 cookie jar 脏写 + 服务端压力放大。
func TestActivateSessionIfNeeded_BackoffHundredConcurrent(t *testing.T) {
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
		WithSessionBackoff(time.Hour), // 大窗口保证所有 goroutine 命中
	)

	const goroutines = 100
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
	// 步骤 4 只应被执行 1 次，而不是 100 次
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4（getMyInfo）期望 1 次 HTTP 请求，实际 %d — backoff 未抑制", got)
	}
}
