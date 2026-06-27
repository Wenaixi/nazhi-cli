// Package client 白盒测试：WithLogger nil 守卫。
package client

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestWithLogger_NilRejected 回归测试（A1）：
// WithLogger(nil) 必须被拒绝，warn 提醒，保持当前 logger（防止 nil 覆盖后
// 后续 c.logger.Warn/Debug/Error 全部 nil pointer panic）。
//
// 设计一致：与 WithTimeout(D1) / WithHTTPClient(F8) 的 nil 拒绝守卫对称。
func TestWithLogger_NilRejected(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	c := &Client{
		logger: logger,
	}

	WithLogger(nil)(c)
	if c.logger != logger {
		t.Errorf("WithLogger(nil) 应被拒绝，保持原 logger，实际被替换")
	}
	if !strings.Contains(logBuf.String(), "nil") || !strings.Contains(logBuf.String(), "WithLogger") {
		t.Errorf("应 warn 包含 'nil' 和 'WithLogger'，实际 log：%s", logBuf.String())
	}
}
