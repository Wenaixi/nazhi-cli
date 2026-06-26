// Package client 内部白盒测试。
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

// TestWithSessionBackoff_SetsDuration 验证 WithSessionBackoff(d) 正常路径：
// 设置正数 d 后，c.sessionBackoff 字段被更新；首次失败后窗口期内同 token
// 第二次激活走 backoff 抑制路径，error 包装 ErrSessionBackoff。
func TestWithSessionBackoff_SetsDuration(t *testing.T) {
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&failureCount, 1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer failSrv.Close()

	c, _ := New(
		WithBaseURL(failSrv.URL),
		WithTimeout(5*time.Second),
		WithSessionBackoff(10*time.Second), // B5 测试目标：通过 Option 设置 10s
	)
	if c.sessionBackoff != 10*time.Second {
		t.Fatalf("WithSessionBackoff(10s) 后 sessionBackoff = %v，期望 10s", c.sessionBackoff)
	}

	// 第一次激活：步骤 4 失败
	if _, err := c.ActivateSession(context.Background(), "token-X"); err == nil {
		t.Fatal("第一次激活在失败 server 上应返回 error")
	}
	// 同 token 立即再试：因 sessionBackoff=10s > 0 → 命中 backoff 抑制
	_, errSecond := c.ActivateSession(context.Background(), "token-X")
	if errSecond == nil {
		t.Fatal("第二次激活应被 backoff 抑制，实际 nil")
	}
	if !errors.Is(errSecond, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff 哨兵，实际 err=%v", errSecond)
	}
}

// TestWithSessionBackoff_ZeroRejected 验证 WithSessionBackoff(0) 被拒绝：
// 与 WithTimeout(0) 对称守卫 — 0 表示「禁用 backoff」，可能让调用方忘记恢复。
// 必须 warn + 不修改字段（保持原 defaultSessionBackoff 行为或上次设置）。
func TestWithSessionBackoff_ZeroRejected(t *testing.T) {
	c, _ := New(
		WithBaseURL("http://127.0.0.1:1"), // 不实际发请求
		WithTimeout(time.Second),
	)
	// 初始 sessionBackoff 默认为 0（=使用 defaultSessionBackoff）
	originalBackoff := c.sessionBackoff

	WithSessionBackoff(0)(c)

	if c.sessionBackoff != originalBackoff {
		t.Errorf("WithSessionBackoff(0) 应保留原值 %v，实际 %v", originalBackoff, c.sessionBackoff)
	}
}

// TestWithSessionBackoff_NegativeRejected 验证 WithSessionBackoff(-1s) 被拒绝。
func TestWithSessionBackoff_NegativeRejected(t *testing.T) {
	c, _ := New(
		WithBaseURL("http://127.0.0.1:1"),
		WithTimeout(time.Second),
	)
	originalBackoff := c.sessionBackoff

	WithSessionBackoff(-1 * time.Second)(c)

	if c.sessionBackoff != originalBackoff {
		t.Errorf("WithSessionBackoff(-1s) 应保留原值 %v，实际 %v", originalBackoff, c.sessionBackoff)
	}
}

// failureCount 是测试用全局计数器（被多个 backoff 测试复用，单独定义避免冲突）
var failureCount int32
