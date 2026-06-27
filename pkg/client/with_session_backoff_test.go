package client

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWithSessionBackoff_PositiveAccepted 验证 WithSessionBackoff(d>0) 设置字段。
func TestWithSessionBackoff_PositiveAccepted(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger: logger,
		sm:     &sessionManager{},
	}

	if c.sm.backoff != 0 {
		t.Fatalf("初始 sm.backoff 应 = 0，实际 %v", c.sm.backoff)
	}

	WithSessionBackoff(10 * time.Second)(c)
	if c.sm.backoff != 10*time.Second {
		t.Errorf("WithSessionBackoff(10s) 应设置 sm.backoff = 10s，实际 %v", c.sm.backoff)
	}

	if logBuf.Len() > 0 {
		t.Errorf("WithSessionBackoff(d>0) 不应输出 warn，实际 log: %s", logBuf.String())
	}
}

func TestWithSessionBackoff_ZeroRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger: logger,
		sm:     &sessionManager{backoff: 15 * time.Second},
	}

	WithSessionBackoff(0)(c)
	if c.sm.backoff != 15*time.Second {
		t.Errorf("WithSessionBackoff(0) 应被拒绝保持原值 15s，实际 %v", c.sm.backoff)
	}
	if !strings.Contains(logBuf.String(), "0") {
		t.Errorf("应 warn 包含 '0'，实际 log: %s", logBuf.String())
	}
}

func TestWithSessionBackoff_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger: logger,
		sm:     &sessionManager{backoff: 20 * time.Second},
	}

	WithSessionBackoff(-1 * time.Second)(c)
	if c.sm.backoff != 20*time.Second {
		t.Errorf("WithSessionBackoff(-1s) 应被拒绝保持原值 20s，实际 %v", c.sm.backoff)
	}
	if !strings.Contains(logBuf.String(), "负数") && !strings.Contains(logBuf.String(), "负") {
		t.Errorf("应 warn 包含 '负'，实际 log: %s", logBuf.String())
	}
}

// TestActivateWithBackoffCheck_UsesConfiguredBackoff 验证 activateWithBackoffCheck
// 实际消费 sm.backoff 字段——而非硬编码 5s 默认值。
func TestActivateWithBackoffCheck_UsesConfiguredBackoff(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	errSentinel := errors.New("simulated last activation failure")
	c := &Client{
		logger: logger,
		sm: &sessionManager{
			backoff:         1 * time.Hour,
			lastErr:         errSentinel,
			lastAttempt:     time.Now(),
			lastFailedToken: "test-token",
			mu:              sync.Mutex{},
		},
	}

	c.sm.mu.Lock()
	_, err := c.activateWithBackoffCheck(context.Background(), "test-token")
	c.sm.mu.Unlock()

	if err == nil {
		t.Fatal("1 小时 backoff 窗口内同 token 应被抑制返回错误，实际 nil")
	}
	if !errors.Is(err, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff，err=%v", err)
	}
}
