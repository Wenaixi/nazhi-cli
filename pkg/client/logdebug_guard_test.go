// Package client 白盒测试：logDebug nil + LevelEnabled 守卫。
package client

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestLogDebug_NilLogger_NoPanic 回归测试（A2）：
// c.logger 为 nil 时 logDebug 不应 panic，静默返回。
func TestLogDebug_NilLogger_NoPanic(t *testing.T) {
	c := &Client{
		logger: nil, // 模拟 logger 为 nil（理论上不会发生，但防 WithLogger(nil) 泄漏）
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("logDebug 在 logger == nil 时 panic: %v", r)
		}
	}()

	// 不应 panic
	c.logDebug("test format: %s %d", "hello", 42)
}

// TestLogDebug_LevelWarn_Silent 回归测试（A3）：
// LevelWarn 下 logDebug 应跳过（不调用 handler），避免 hot loop 中
// 不必要的 fmt.Sprintf 分配（OCR 99 张图 × 5 个 logDebug = 500+ 次 wasted alloc）。
func TestLogDebug_LevelWarn_Silent(t *testing.T) {
	var handlerCalled bool
	handler := slog.New(slog.NewTextHandler(&bytes.Buffer{},
		&slog.HandlerOptions{Level: slog.LevelWarn},
	))

	// 包装 handler 使 Debug 调用被记录
	c := &Client{}
	instrumented := slog.New(&callRecorderHandler{
		next:   handler.Handler(),
		called: &handlerCalled,
	})
	c.logger = instrumented

	c.logDebug("should not appear: %s", "secret")
	if handlerCalled {
		t.Errorf("LevelWarn 下 logDebug 不应调用 handler，但实际调用了")
	}
}

// callRecorderHandler 包装 slog.Handler，记录是否有 Handle 调用。
type callRecorderHandler struct {
	next   slog.Handler
	called *bool
}

func (h *callRecorderHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *callRecorderHandler) Handle(ctx context.Context, r slog.Record) error {
	*h.called = true
	return h.next.Handle(ctx, r)
}

func (h *callRecorderHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &callRecorderHandler{next: h.next.WithAttrs(attrs), called: h.called}
}

func (h *callRecorderHandler) WithGroup(name string) slog.Handler {
	return &callRecorderHandler{next: h.next.WithGroup(name), called: h.called}
}

// TestLogDebug_DebugEnabled_CallsHandler 正向验证（A3）：
// LevelDebug 下 logDebug 正常调用 handler。
func TestLogDebug_DebugEnabled_CallsHandler(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf,
		&slog.HandlerOptions{Level: slog.LevelDebug},
	))

	c := &Client{logger: logger}
	c.logDebug("visible: %s", "data")

	if !strings.Contains(logBuf.String(), "visible") || !strings.Contains(logBuf.String(), "data") {
		t.Errorf("LevelDebug 下 logDebug 应输出日志，实际 log: %s", logBuf.String())
	}
}
