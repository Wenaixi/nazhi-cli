package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestActivateFailedToken_DoesNotClearOtherTokenCache_SameClient 验证在同一 Client
// 上先激活 token-A 成功、再激活 token-B 失败时，token-A 的缓存不被清除。
func TestActivateFailedToken_DoesNotClearOtherTokenCache_SameClient(t *testing.T) {
	var mu sync.Mutex
	step4Fail := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"success"}`))
		case "/api/studentInfo/getMyInfo":
			mu.Lock()
			fail := step4Fail
			mu.Unlock()
			if fail {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":0,"msg":"服务降级"}`))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
			}
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.ActivateSession(context.Background(), "tok-A")
	if err != nil {
		t.Fatalf("token-A 激活失败: %v", err)
	}
	if info == nil || info.Name != "张三" {
		t.Fatalf("token-A 激活后 info 异常: %+v", info)
	}
	if c.sm.LoadToken() != "tok-A" {
		t.Fatalf("sm token 应为 tok-A, 实际 %v", c.sm.LoadToken())
	}
	cached := c.sm.GetCachedUserInfo()
	if cached == nil || cached.Name != "张三" {
		t.Fatalf("token-A 激活后 sm cachedUserInfo 应有数据: %+v", cached)
	}

	mu.Lock()
	step4Fail = true
	mu.Unlock()

	_, err = c.ActivateSession(context.Background(), "tok-B")
	if err == nil {
		t.Fatal("token-B 激活应失败，但返回 nil")
	}

	cached = c.sm.GetCachedUserInfo()
	if cached == nil {
		t.Fatal("F2 回归：token-B 失败不应清除 token-A 的 cachedUserInfo")
	}
	if cached.Name != "张三" {
		t.Errorf("cachedUserInfo.Name 期望 '张三', 实际 %q", cached.Name)
	}
	if c.sm.LoadToken() != "tok-A" {
		t.Errorf("sm token 应仍为 tok-A（token-B 未成功）, 实际 %v", c.sm.LoadToken())
	}
}
