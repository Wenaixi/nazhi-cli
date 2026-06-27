package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestActivateSessionIfNeeded_ConcurrentSameToken 回归测试：N 个 goroutine
// 并发调用 activateSessionIfNeeded(token)，验证：
// 1. 不发生 race
// 2. 4 步激活请求不被重复执行（每个端点的请求次数恒定）
// 3. sm token 在所有 goroutine 返回前/后均为同一 token
func TestActivateSessionIfNeeded_ConcurrentSameToken(t *testing.T) {
	var (
		step1Count int32
		step2Count int32
		step3Count int32
		step4Count int32
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			atomic.AddInt32(&step1Count, 1)
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			if r.Header.Get("Referer") != "" && strings.Contains(r.Header.Get("Referer"), "/homepage") {
				atomic.AddInt32(&step2Count, 1)
			} else {
				atomic.AddInt32(&step3Count, 1)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.activateSessionIfNeeded(context.Background(), "shared-token"); err != nil {
				t.Errorf("activateSessionIfNeeded 失败: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&step1Count); got != 1 {
		t.Errorf("步骤 1 期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step2Count); got != 1 {
		t.Errorf("步骤 2 期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step3Count); got != 1 {
		t.Errorf("步骤 3 期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4 期望 1 次，实际 %d", got)
	}

	if c.sm.LoadToken() != "shared-token" {
		t.Errorf("sm token = %q, 期望 %q", c.sm.LoadToken(), "shared-token")
	}
}

// TestActivateSessionIfNeeded_ConcurrentDifferentTokens 回归测试：N 个 goroutine
// 持不同 token 并发调用 activateSessionIfNeeded，验证：
// 1. 不发生 race
// 2. 持锁激活保证不同 token 串行执行 4 步，无并发写 cookie jar 污染
// 3. sm token 最终为某次成功调用的 token
func TestActivateSessionIfNeeded_ConcurrentDifferentTokens(t *testing.T) {
	var step1Count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			atomic.AddInt32(&step1Count, 1)
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	const goroutines = 8
	tokens := []string{
		"tok-A", "tok-B", "tok-C", "tok-D",
		"tok-E", "tok-F", "tok-G", "tok-H",
	}
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			if _, err := c.activateSessionIfNeeded(context.Background(), tokens[i]); err != nil {
				t.Errorf("token=%s activateSessionIfNeeded 失败: %v", tokens[i], err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&step1Count); got != int32(goroutines) {
		t.Errorf("不同 token 步骤 1 期望 %d 次，实际 %d", goroutines, got)
	}
	finalToken := c.sm.LoadToken()
	found := false
	for _, tok := range tokens {
		if finalToken == tok {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sm token=%q 不在预期 token 列表中", finalToken)
	}
}
