// Package client 内部白盒测试。
package client

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestWithTimeout_NegativeRejected 回归测试：WithTimeout(-1) 必须被拒绝，
// 保持当前 Timeout 值（防止把超时改成无效负数）。
//
// 历史 bug：WithTimeout 不校验 d，对 d=0/d<0 静默接受。d=0 让请求永久
// 挂起，d<0 是非法值（Go time.Duration 负数无意义但合法）。
func TestWithTimeout_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := New(
		WithTimeout(15*time.Second),
		WithLogger(logger),
	)
	if c.http.Timeout != 15*time.Second {
		t.Fatalf("初始 Timeout 应 = 15s，实际 %v", c.http.Timeout)
	}

	// 再次 WithTimeout(-1) 应被拒绝
	WithTimeout(-1 * time.Second)(c)
	if c.http.Timeout != 15*time.Second {
		t.Errorf("WithTimeout(-1s) 应被拒绝，实际 Timeout=%v", c.http.Timeout)
	}
	if !strings.Contains(logBuf.String(), "负数超时被拒绝") {
		t.Errorf("应 warn '负数超时被拒绝'，实际 log：%s", logBuf.String())
	}
}

// TestWithTimeout_ZeroWarns 验证 WithTimeout(0) 仍设置但 warn 提示风险。
func TestWithTimeout_ZeroWarns(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 白盒构造：先放好 logger，再调 WithTimeout，确保 warn 走 logBuf
	c := &Client{
		http:   newHTTPClient(),
		logger: logger,
	}
	WithTimeout(0)(c)
	if c.http.Timeout != 0 {
		t.Errorf("WithTimeout(0) 应设置 Timeout=0（net/http 无超时），实际 %v", c.http.Timeout)
	}
	if !strings.Contains(logBuf.String(), "无超时") {
		t.Errorf("应 warn '无超时' 风险，实际 log：%s", logBuf.String())
	}
}
