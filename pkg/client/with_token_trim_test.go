// Package client 内部白盒测试。
package client

import (
	"io"
	"log/slog"
	"testing"
)

// TestWithToken_TrimsWhiteSpace 验证传入带空格 token 时 WithToken 存储的是修剪后的值。
//
// 历史 bug：WithToken 用 strings.TrimSpace(token) == "" 校验空字符串，
// 但存储时用原始值 c.pendingToken = token（含前/后空白）。
// 后续 New() 末尾 syncCookieToken 写入 cookie 时 value 含有空白，
// 导致服务端解析出带空格的畸形 token，鉴权失败。
//
// 修复后：c.pendingToken = strings.TrimSpace(token)，与校验逻辑对称。
func TestWithToken_TrimsWhiteSpace(t *testing.T) {
	c := &Client{
		pendingToken: "",
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}

	WithToken("  eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc  ")(c)
	want := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc"
	if c.pendingToken != want {
		t.Errorf("WithToken 应存储修剪后的值，实际 %q", c.pendingToken)
	}
}
