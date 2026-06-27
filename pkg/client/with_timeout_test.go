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
// 历史 bug：WithTimeout 不校验 d，对 d=0/d<0 静默接受。d=0 让请求永久
// 挂起，d<0 是非法值（Go time.Duration 负数无意义但合法）。
func TestWithTimeout_NegativeRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c, _ := New(
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

// TestWithTimeout_ZeroWarns 验证 WithTimeout(0) 被拒绝（保留原值）+ warn 提示风险。
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

// TestWithTimeout_ZeroDoesNotOverwriteExisting 回归测试（F9）：
// WithTimeout(0) 在已设置正数超时时**不应**清零已有超时。
// 历史 bug：WithTimeout(0) 仅 warn 但仍执行 c.http.Timeout = 0，
// 静默破坏调用方已配置的 15s 超时为"无超时"——后续请求可能永久挂起。
// 修复后 d==0 必须阻断赋值（与 d<0 行为对齐：拒绝 + warn 保持原值）。
func TestWithTimeout_ZeroDoesNotOverwriteExisting(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		http:   newHTTPClient(),
		logger: logger,
	}
	// 先设 15s
	WithTimeout(15 * time.Second)(c)
	if c.http.Timeout != 15*time.Second {
		t.Fatalf("初始 Timeout 应 = 15s，实际 %v", c.http.Timeout)
	}

	// 再 WithTimeout(0) 不应清零
	WithTimeout(0)(c)
	if c.http.Timeout != 15*time.Second {
		t.Errorf("WithTimeout(0) 不应清零已有超时，实际 Timeout=%v", c.http.Timeout)
	}
	if !strings.Contains(logBuf.String(), "无超时") {
		t.Errorf("应 warn '无超时' 风险，实际 log：%s", logBuf.String())
	}
}

// TestWithTimeout_NilHTTPWarns 回归测试（F9）：
// WithTimeout 在 c.http == nil 时不应静默 return——至少 warn 让用户感知。
// 历史 bug：WithTimeout 在 c.http == nil 时静默 return，调用方无法
// 知道 timeout 未生效。WithHTTPClient(nil) 是触发 c.http == nil 的
// 唯一外部路径——属于误用但需要被看见而非吞掉。
func TestWithTimeout_NilHTTPWarns(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	c := &Client{
		http:   nil, // 模拟 WithHTTPClient(nil) 之后的状态
		logger: logger,
	}

	// 不应 panic，应至少输出 warn
	WithTimeout(5 * time.Second)(c)

	if c.http != nil {
		t.Errorf("c.http==nil 路径不应创建 http client")
	}
	if !strings.Contains(logBuf.String(), "c.http 为 nil") {
		t.Errorf("应 warn 'c.http 为 nil'，实际 log：%s", logBuf.String())
	}
}
