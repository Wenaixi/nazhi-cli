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
		WithBaseURL(failSrv.URL),
		WithTimeout(5*time.Second),
	)
	c.sm.backoff = time.Hour

	if _, err := c.ActivateSession(context.Background(), "token-A"); err == nil {
		t.Fatal("第一阶段：token-A 在失败 server 上应返回 error")
	}
	if c.sm.lastErr == nil {
		t.Fatal("第一阶段后：sm.lastErr 应被缓存，实际 nil")
	}

	c.baseURL = okSrv.URL
	if _, err := c.ActivateSession(context.Background(), "token-B"); err != nil {
		t.Fatalf("第二阶段：token-B 在成功 server 上激活应成功，实际: %v", err)
	}
	if c.sm.LoadToken() != "token-B" {
		t.Errorf("token-B 成功后 sm token 应 = \"token-B\"，实际 %q", c.sm.LoadToken())
	}
	if c.sm.lastErr != nil {
		t.Errorf("token-B 成功后 sm.lastErr 应清零，实际 %v", c.sm.lastErr)
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
	c.sm.backoff = time.Hour

	if _, err := c.ActivateSession(context.Background(), "token-X"); err == nil {
		t.Fatal("第一阶段：token-X 在失败 server 上应返回 error")
	}
	_, errSecond := c.ActivateSession(context.Background(), "token-X")
	if errSecond == nil {
		t.Error("同 token 在 backoff 窗口内应仍返回缓存错误，实际 nil")
	}
	if !errors.Is(errSecond, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff 哨兵，err=%v", errSecond)
	}
}
