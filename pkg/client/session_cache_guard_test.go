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
//
// RED 阶段（修复前）：activateWithBackoffCheck 失败路径无条件
// c.cachedUserInfo = nil → 即使 token-B 失败，token-A 的缓存也被清空。
// GREEN 阶段（修复后）：只有 sessionToken == token 时才清除缓存。
//
// 测试步骤：
//  1. server 先正常响应 token-A 的激活（4 步全部返回 code=1）
//  2. 然后切换响应：对 token-B 的步骤 4 返回 code=0 使激活失败
//  3. token-B 失败 → cachedUserInfo 不应被清（sessionToken 仍为 tok-A）
//  4. 验证 cachedUserInfo 仍然存在且 Name == "张三"
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

	// 步骤 1：token-A 激活成功
	info, err := c.ActivateSession(context.Background(), "tok-A")
	if err != nil {
		t.Fatalf("token-A 激活失败: %v", err)
	}
	if info == nil || info.Name != "张三" {
		t.Fatalf("token-A 激活后 info 异常: %+v", info)
	}
	if c.sessionToken.Load() != "tok-A" {
		t.Fatalf("sessionToken 应为 tok-A, 实际 %v", c.sessionToken.Load())
	}
	if c.cachedUserInfo == nil || c.cachedUserInfo.Name != "张三" {
		t.Fatalf("token-A 激活后 cachedUserInfo 应有数据: %+v", c.cachedUserInfo)
	}

	// 步骤 2：切换到 token-B 并使 B 的激活失败
	mu.Lock()
	step4Fail = true
	mu.Unlock()

	_, err = c.ActivateSession(context.Background(), "tok-B")
	if err == nil {
		t.Fatal("token-B 激活应失败，但返回 nil")
	}

	// 核心断言：cachedUserInfo 应保留 token-A 的数据
	// 修复前：无条件 =nil → 丢失；修复后：sessionToken(tok-A) != token(tok-B) → 保留
	if c.cachedUserInfo == nil {
		t.Fatal("F2 回归：token-B 失败不应清除 token-A 的 cachedUserInfo")
	}
	if c.cachedUserInfo.Name != "张三" {
		t.Errorf("cachedUserInfo.Name 期望 '张三', 实际 %q", c.cachedUserInfo.Name)
	}

	// sessionToken 应保持为 tok-A（token-B 从未成功）
	if c.sessionToken.Load() != "tok-A" {
		t.Errorf("sessionToken 应仍为 tok-A（token-B 未成功）, 实际 %v", c.sessionToken.Load())
	}
}
