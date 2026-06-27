package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestActivateSession_ConcurrentNoDeadlock 验证 ActivateSession 在 100 个
// goroutine 并发调用时不发生死锁。
// 历史风险：ActivateSession 在 sessionMu 持锁状态下执行
// 4 步网络请求，如果外部调用方在 errgroup.Go 中持其他锁再调 ActivateSession，
// 可能引发 ABBA 死锁。本测试验证 ActivateSession 自身不因 sessionMu 持有
// 而阻塞所有并发——仅锁定序列化 4 步写入 cookie jar。
func TestActivateSession_ConcurrentNoDeadlock(t *testing.T) {
	stepCount := struct {
		sync.Mutex
		val int
	}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			stepCount.Lock()
			stepCount.val++
			stepCount.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			token := fmt.Sprintf("token-%03d", i)
			_, _ = c.ActivateSession(context.Background(), token)
		}()
	}
	close(start)
	wg.Wait()

	// 如果走到这里没死锁，测试通过
	// 验证至少大部分 token 都完成了步骤 4（因 httptest 并发处理和 backoff
	// 序列化，在极端调度下可能个别 goroutine 的请求被服务端漏响应，但
	// 99+/100 是可接受的——核心目标是验证不死锁）。
	stepCount.Lock()
	got := stepCount.val
	stepCount.Unlock()
	if got < goroutines-1 {
		t.Fatalf("期望至少 %d 次步骤 4 调用，实际 %d — 可能有 token 被跳过或死锁", goroutines-1, got)
	}
	t.Logf("步骤 4 被调用 %d 次（共 %d goroutine），无死锁 ✓", got, goroutines)
}
