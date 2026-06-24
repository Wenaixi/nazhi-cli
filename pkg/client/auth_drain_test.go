// Package client 内部白盒测试。
//
// F1: pkg/client/auth.go Login 缺 drain+close — 回归测试。
//
// 历史 bug：Login 函数用 `defer httpResp.Body.Close()` 但没 drain body，
// 6 个 early-return 路径（130/137/147/167/171/193）通过这个 defer，
// 未 drain 的 body 强制 net/http 关闭 TCP 连接，无法归还 keep-alive 池。
//
// 修复后：defer drain + close（参考 request.go:132-136 已有的模式），
// 让 net/http 把连接归还 keep-alive 池。
//
// 验证策略：使用真实 httptest.Server + 注入 bug 的方式。
//
// 巧妙方案：让 server 返回的 response 在 Login 的 defer 触发瞬间，
// body 还有可读字节。这要求 io.ReadAll 不读完所有字节——只能靠错误终止。
//
// 我们的 mock body 设计：
//   - body.Read 第一次返回 (0, io.ErrUnexpectedEOF) → io.ReadAll 立即出错
//   - 之后 body 仍声明有 N 字节 → 只有 drain 才会读走这些字节
//   - Close 时记录 "被 drain 读走的字节数"
//
// 验证：修复前 defer 只有 Close → drained=0；修复后 defer drain → drained=N。
package client

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// readTrackingBody 是 Login 测试用的 mock body。
//   - 第 1 次 Read：返回 (0, io.ErrUnexpectedEOF) —— 让 io.ReadAll 立即失败
//   - 之后 Read：返回 remainingBytes 切片内容（只有 drain 才会调用）
//   - 每次 Read 都把 n 加到 *readByDrain（drain 标识）
type readTrackingBody struct {
	remaining   []byte
	readByDrain *int32 // 累计 Read 返回的字节数（无论 io.ReadAll 还是 drain）
	firstRead   bool   // 第一次 Read 返回 (0, io.ErrUnexpectedEOF)
}

func (b *readTrackingBody) Read(p []byte) (int, error) {
	if b.firstRead {
		b.firstRead = false
		return 0, io.ErrUnexpectedEOF
	}
	if len(b.remaining) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.remaining)
	b.remaining = b.remaining[n:]
	atomic.AddInt32(b.readByDrain, int32(n))
	return n, nil
}

func (b *readTrackingBody) Close() error { return nil }

// readTrackingRT 是 http.RoundTripper mock。
// 它只对 /teacher/auth/studentLogin/validate (Login 主路径) 返回带 remaining 的 body。
// 其他路径返回正常响应，让 Login 走通到 validate。
type readTrackingRT struct {
	validateReadBytes *int32
	calls             *int32
}

func (rt *readTrackingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(rt.calls, 1)

	// Login 主路径：返回带 100 字节 remaining 的 body
	// 第 1 次 Read 返回 (0, io.ErrUnexpectedEOF) → io.ReadAll 立即失败
	// Login 走 line 137 错误路径 → defer Close 触发
	// 修复前：defer 只 Close，剩余 100 字节从未被读 → 连接被强制关闭
	// 修复后：defer drain 调 io.Copy(io.Discard, body) → 100 字节被读完
	if strings.Contains(req.URL.Path, "validate") &&
		!strings.Contains(req.URL.Path, "validateCaptcha") &&
		req.Method == http.MethodPost {
		body := &readTrackingBody{
			remaining:   bytes.Repeat([]byte{'D'}, 100),
			readByDrain: rt.validateReadBytes,
			firstRead:   true,
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       body,
			Header:     http.Header{"Content-Length": []string{"100"}},
			Request:    req,
		}, nil
	}

	// InitSession / captcha / validateCaptcha：返回正常 200
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":1,"msg":"成功"}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

func (rt *readTrackingRT) Close() error { return nil }

// ─── 测试 ───

// TestLogin_DrainsBody_On200UnexpectedEOFPath 验证 Login 在 HTTP 200
// + body io.ErrUnexpectedEOF 路径上，必须 drain body 才能让连接归还 keep-alive。
//
// 场景：server 返回 200 + body 声明 100 字节但立即 io.ErrUnexpectedEOF。
// io.ReadAll 失败 → Login 走 line 137 错误返回 → defer Close() 触发。
//
// 修复前：defer httpResp.Body.Close() → 连接被强制关闭（drainedBytes == 0）。
// 修复后：defer { io.Copy(io.Discard, body); body.Close() } → drainedBytes == 100。
func TestLogin_DrainsBody_On200UnexpectedEOFPath(t *testing.T) {
	var validateReadBytes int32
	var calls int32

	rt := &readTrackingRT{
		validateReadBytes: &validateReadBytes,
		calls:             &calls,
	}

	c := &Client{
		ssoBaseURL: "http://mock-sso",
		baseURL:    "http://mock-sso",
		uploadURL:  "http://mock-sso",
		http: &http.Client{
			Transport: rt,
		},
		logger: slog.New(slog.DiscardHandler),
		ocr:    &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173", // 跳过 GetSchoolID
	})

	if err == nil {
		t.Fatal("期望 io.ReadAll 失败，实际 nil")
	}
	// 错误应包含 io.ErrUnexpectedEOF（被 fmt.Errorf %w 包装）
	if !strings.Contains(err.Error(), io.ErrUnexpectedEOF.Error()) {
		t.Errorf("期望 wrap io.ErrUnexpectedEOF，实际: %v", err)
	}

	// 关键断言：validate body 被读取的总字节数 = 100（drain 阶段读走 100 字节）
	// 修复前：defer 只 Close，drain 阶段无 Read 调用 → readBytes < 100
	// 修复后：defer drain 调 io.Copy → readBytes = 100
	readBytes := atomic.LoadInt32(&validateReadBytes)
	if readBytes != 100 {
		t.Errorf("期望 validate body 被读完 100 字节（drain 让连接归还 keep-alive），实际 %d 字节（连接被强制关闭）", readBytes)
	}
}

// ─── 兜底 ───

// TestLogin_RealServer_KeepsAlive 占位 - keep-alive 行为依赖 net/http 内部计时，
// 容易 flaky，主要靠 TestLogin_DrainsBody_On200UnexpectedEOFPath 保证 F1 修复。
