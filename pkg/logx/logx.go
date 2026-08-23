// Package logx 提供结构化日志、脱敏与 traceId 上下文薄封装。
package logx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// 上下文携带 traceId 的 key。
type ctxKey struct{}

// WithTraceID 将 traceId 写入 context。
func WithTraceID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// TraceIDFrom 从 context 读取 traceId。
func TraceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// NewTraceID 生成 12 字符 hex 随机 traceId。
func NewTraceID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ParseLevel 解析日志级别字符串。
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelWarn, fmt.Errorf("未知 log level %q 期望 debug/info/warn/error", s)
	}
}

// ParseFormat 解析日志格式字符串。
func ParseFormat(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "text", "":
		return "text", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("未知 log format %q 期望 text/json", s)
	}
}

// multiWriter 将写入并发安全地扇出到多个 writer。
type multiWriter struct {
	mu sync.Mutex
	ws []io.Writer
}

func (m *multiWriter) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var firstErr error
	for _, w := range m.ws {
		if _, err := w.Write(p); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return 0, firstErr
	}
	return len(p), nil
}

// NewLogger 创建 slog.Logger，按 level 与 format 配置，输出到 writers。
// 未传 writers 时默认输出到 os.Stderr。
func NewLogger(level slog.Level, format string, writers ...io.Writer) *slog.Logger {
	if len(writers) == 0 {
		writers = []io.Writer{os.Stderr}
	}
	var w io.Writer
	if len(writers) == 1 {
		w = writers[0]
	} else {
		w = &multiWriter{ws: writers}
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

// NewFileWriter 打开日志文件用于追加写入。
// ponytail: 无轮转，文件超过 50MB 时再引入 lumberjack。
func NewFileWriter(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}