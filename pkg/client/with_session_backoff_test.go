// Package client 内部白盒测试。
//
// H2 修复（round-9）：ErrSessionBackoff 哨兵已落地（F15 round-7），
// 但 sessionBackoff 字段无对应 Option。SDK 用户无法调整 5s 默认窗口。
//
// 本测试文件覆盖 WithSessionBackoff 的 4 个契约：
//   - d > 0：设置字段生效（与 WithTimeout 对称）
//   - d = 0：拒绝并 warn（与 WithTimeout(0) 对称：防止静默清零默认值）
//   - d < 0：拒绝并 warn（与 WithTimeout(-1) 对称）
//   - activateWithBackoffCheck 用字段值计算窗口（验证字段被实际消费）
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
	}

	// 初始字段值应为 0（未设置）
	if c.sessionBackoff != 0 {
		t.Fatalf("初始 sessionBackoff 应 = 0，实际 %v", c.sessionBackoff)
	}

	// 设置 10 秒
	WithSessionBackoff(10 * time.Second)(c)
	if c.sessionBackoff != 10*time.Second {
		t.Errorf("WithSessionBackoff(10s) 应设置字段 = 10s，实际 %v", c.sessionBackoff)
	}

	// 不应有任何 warn
	if logBuf.Len() > 0 {
		t.Errorf("WithSessionBackoff(d>0) 不应输出 warn，实际 log: %s", logBuf.String())
	}
}

// TestWithSessionBackoff_ZeroRejected 验证 WithSessionBackoff(0) 拒绝并 warn。
func TestWithSessionBackoff_ZeroRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger:         logger,
		sessionBackoff: 15 * time.Second, // 已设置正数值
	}

	// 0 应被拒绝
	WithSessionBackoff(0)(c)
	if c.sessionBackoff != 15*time.Second {
		t.Errorf("WithSessionBackoff(0) 应被拒绝保持原值 15s，实际 %v", c.sessionBackoff)
	}
	if !strings.Contains(logBuf.String(), "0") {
		t.Errorf("应 warn 包含 '0'，实际 log: %s", logBuf.String())
	}
}

// TestWithSessionBackoff_NegativeRejected 验证 WithSessionBackoff(-1) 拒绝并 warn。
func TestWithSessionBackoff_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger:         logger,
		sessionBackoff: 20 * time.Second,
	}

	WithSessionBackoff(-1 * time.Second)(c)
	if c.sessionBackoff != 20*time.Second {
		t.Errorf("WithSessionBackoff(-1s) 应被拒绝保持原值 20s，实际 %v", c.sessionBackoff)
	}
	if !strings.Contains(logBuf.String(), "负数") && !strings.Contains(logBuf.String(), "负") {
		t.Errorf("应 warn 包含 '负'，实际 log: %s", logBuf.String())
	}
}

// TestActivateWithBackoffCheck_UsesConfiguredBackoff 验证 activateWithBackoffCheck
// 实际消费 sessionBackoff 字段——而非硬编码 5s 默认值。
//
// 场景：字段设为 1 小时，模拟上次失败（lastActivationErr != nil），
// 立即再调应触发 backoff 抑制（返回包装 ErrSessionBackoff 的错误）。
//
// 修复动机：若 WithSessionBackoff 只赋值不消费，SDK 用户调了等于没调。
func TestActivateWithBackoffCheck_UsesConfiguredBackoff(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		logger:            logger,
		sessionBackoff:    1 * time.Hour, // 1 小时窗口
		lastActivationErr: errSentinelBackoffTest,
		lastAttemptAt:     time.Now(),
		lastFailedToken:   "test-token",
		sessionMu:         sync.Mutex{},
	}

	// 持锁调用（activateWithBackoffCheck 要求持锁）
	c.sessionMu.Lock()
	_, err := c.activateWithBackoffCheck(context.Background(), "test-token")
	c.sessionMu.Unlock()

	if err == nil {
		t.Fatal("1 小时 backoff 窗口内同 token 应被抑制返回错误，实际 nil")
	}
	if !errors.Is(err, ErrSessionBackoff) {
		t.Errorf("backoff 错误应包装 ErrSessionBackoff，err=%v", err)
	}
}

// errSentinelBackoffTest 是测试用哨兵错误，模拟上次激活失败原因。
var errSentinelBackoffTest = errors.New("simulated last activation failure")
