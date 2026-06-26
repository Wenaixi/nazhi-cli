package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestActivateSessionIfNeeded_BackoffIsScopedToToken 回归测试 F15：
// 上次激活失败后，backoff 缓存键必须包含 token 维度。同一 Client 切换 token
// 重新激活时，backoff 不应命中上次失败的缓存（避免 stale error 被错误 propagate）。
//
// 失败场景（修复前）：
//  1. Client 用 token-A 激活，步骤 4（getMyInfo）失败 → lastActivationErr 缓存 A 的错误
//  2. 500ms 后用户切换 token-B 调 ActivateSession（token 变化，sessionToken != B）
//  3. 旧逻辑 backoff 命中窗口直接返回 A 的缓存错误（stale propagate）
//  4. B token 实际可行，错误地被抑制 → 用户看不到真实结果
//
// 修复后：lastFailedToken != token 时跳过 backoff 检查，实际尝试激活 token-B。
//
// 内部测试（package client）：可直接访问 sessionBackoff / lastFailedToken 字段。
func TestActivateSessionIfNeeded_BackoffIsScopedToToken(t *testing.T) {
	var step4Count int32
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer failSrv.Close()

	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&step4Count, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer okSrv.Close()

	c, _ := New(
		WithBaseURL(failSrv.URL), // 先指向失败 server
		WithTimeout(5*time.Second),
	)
	// backoff 设很大（1h），确保如果不修复就会一直命中 backoff
	c.sessionBackoff = time.Hour

	// 1. token-A 在 failSrv 上失败 → lastActivationErr = A_err, lastFailedToken = "token-A"
	if _, err := c.ActivateSession(context.Background(), "token-A"); err == nil {
		t.Fatal("第一阶段：token-A 在失败 server 上应返回 error")
	}
	if c.lastActivationErr == nil {
		t.Fatal("第一阶段后：lastActivationErr 应被缓存，实际 nil")
	}

	// 2. 切换到能成功的 okSrv，调用 token-B 激活
	// 因为 token-B 不同于上次失败的 token-A，backoff 不应命中。
	// 修复前：lastFailedToken 字段不存在 → backoff 命中 → 返回 A 的缓存错误（stale）
	// 修复后：lastFailedToken == "token-A" != "token-B" → 跳过 backoff → 实际尝试
	c.baseURL = okSrv.URL
	if _, err := c.ActivateSession(context.Background(), "token-B"); err != nil {
		t.Fatalf("第二阶段：token-B 在成功 server 上激活应成功，实际: %v", err)
	}
	// 验证：token-B 成功后 sessionToken 应更新为 token-B
	if c.sessionToken.Load() != "token-B" {
		t.Errorf("token-B 成功后 sessionToken 应 = \"token-B\"，实际 %q", c.sessionToken.Load())
	}
	if c.lastActivationErr != nil {
		t.Errorf("token-B 成功后 lastActivationErr 应清零，实际 %v", c.lastActivationErr)
	}
}

// TestActivateSessionIfNeeded_BackoffHitsForSameToken 验证同 token 在 backoff
// 窗口内仍被抑制（确认 lastFailedToken 匹配时 backoff 正常工作）。
func TestActivateSessionIfNeeded_BackoffHitsForSameToken(t *testing.T) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer failSrv.Close()

	c, _ := New(
		WithBaseURL(failSrv.URL),
		WithTimeout(5*time.Second),
	)
	c.sessionBackoff = time.Hour

	if _, err := c.ActivateSession(context.Background(), "token-X"); err == nil {
		t.Fatal("第一阶段：token-X 在失败 server 上应返回 error")
	}
	// 同 token 立即再试（应在 backoff 窗口内被抑制）
	_, errSecond := c.ActivateSession(context.Background(), "token-X")
	if errSecond == nil {
		t.Error("同 token 在 backoff 窗口内应仍返回缓存错误，实际 nil")
	}
	// 关键断言：backoff 错误必须能用 errors.Is(err, ErrSessionBackoff) 识别
	if !errors.Is(errSecond, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff 哨兵（让 SDK 用户能精确判定），err=%v", errSecond)
	}
}
