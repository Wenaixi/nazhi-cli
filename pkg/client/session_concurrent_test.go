// Package client 内部白盒测试。
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
//  1. 不发生 race（-race 检测器必须干净）
//  2. 4 步激活请求不被重复执行（每个端点的请求次数恒定）
//  3. sessionToken 在所有 goroutine 返回前/后均为同一 token
//
// 历史 bug（F3）：原实现采用短锁双重检查模式——line 120 Lock 检查后
// line 125 立即 Unlock，line 127 在无锁状态下执行 4 步网络请求。
// 多个 goroutine 持相同 token 并发进入时全部通过 check，全部执行 4 步，
// 写共享 cookie jar 造成状态机污染。N 个并发 goroutine 触发 4N 步冗余请求。
func TestActivateSessionIfNeeded_ConcurrentSameToken(t *testing.T) {
	var (
		step1Count int32 // GET /
		step2Count int32 // GET /api/studentInfo/getMenu 步骤 2
		step3Count int32 // GET /api/studentInfo/getMenu 步骤 3
		step4Count int32 // GET /api/studentInfo/getMyInfo
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			atomic.AddInt32(&step1Count, 1)
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			// 步骤 2/3 共用路径——通过 Referer 区分计数
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

	// 验证：4 步各只被请求 1 次（持锁激活保证串行化）
	if got := atomic.LoadInt32(&step1Count); got != 1 {
		t.Errorf("步骤 1（首页）期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step2Count); got != 1 {
		t.Errorf("步骤 2（getMenu homepage）期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step3Count); got != 1 {
		t.Errorf("步骤 3（getMenu /home）期望 1 次，实际 %d", got)
	}
	if got := atomic.LoadInt32(&step4Count); got != 1 {
		t.Errorf("步骤 4（getMyInfo）期望 1 次，实际 %d", got)
	}

	// 验证：sessionToken 已被写为 shared-token
	if c.sessionToken.Load() != "shared-token" {
		t.Errorf("sessionToken = %q, 期望 %q", c.sessionToken.Load(), "shared-token")
	}
}

// TestActivateSessionIfNeeded_ConcurrentDifferentTokens 回归测试：N 个 goroutine
// 持不同 token 并发调用 activateSessionIfNeeded，验证：
//  1. 不发生 race
//  2. 持锁激活保证不同 token 串行执行 4 步，无并发写 cookie jar 污染
//  3. sessionToken 最终为某次成功调用的 token（不必是 N 个中的特定一个，
//     但应唯一）
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

	// 验证：每个不同 token 都会触发自己的 4 步（共 N 次步骤 1）
	// 持锁串行化是测试正确性的关键——不持锁则可能丢失部分激活
	if got := atomic.LoadInt32(&step1Count); got != int32(goroutines) {
		t.Errorf("不同 token 步骤 1 期望 %d 次，实际 %d", goroutines, got)
	}
	// sessionToken 必须是 N 个 token 之一
	finalToken := c.sessionToken.Load()
	found := false
	for _, tok := range tokens {
		if finalToken == tok {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sessionToken=%q 不在预期 token 列表中", finalToken)
	}
}

// contains 已被删除（F2 修复）：改用标准库 strings.Contains。
// 原自造 contains 注释称"避免导入 strings 触发额外编译器感知"，
// 与 Go test 文件实践不符——本包 10+ 测试文件已 import strings。
