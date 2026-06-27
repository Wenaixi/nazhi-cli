// Package client 内部白盒测试。
// G3: Login 200 路径 io.ReadAll 错误应包含 status code
// 和已读字节数上下文。
// 历史 bug（auth.go:135-138）：
//
//	bodyBytes, err := io.ReadAll(httpResp.Body)
//	if err != nil {
//	 return nil, fmt.Errorf("Login 读取响应体失败: %w", err)
//	}
//
// 错误信息完全丢失根因上下文：
// - 不知道是哪个 status code（200/302/500/...）的 body 失败
// - 不知道已读了 N 字节才失败（N 字节里可能有 JSON 头部/线索）
// 修复后：错误信息应包含 "status=%d read=%d bytes" 两个上下文，便于排查
// server 端异常（如部分返回 200 但连接 reset、content-length 不符等）。
package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// errAfterBytesBody 是 mock body：先返回 N 字节，再返回 io.ErrUnexpectedEOF。
// 让 io.ReadAll 失败但已读了 N 字节，触发 G3 错误包装逻辑。
type errAfterBytesBody struct {
	remaining []byte
	readByAll *int32
	firstRead bool
}

func (b *errAfterBytesBody) Read(p []byte) (int, error) {
	if b.firstRead && len(b.remaining) > 0 {
		// 第 1 次 Read：返回前 50 字节（readByAll += 50）
		b.firstRead = false
		n := copy(p, b.remaining)
		b.remaining = b.remaining[n:]
		atomic.AddInt32(b.readByAll, int32(n))
		return n, nil
	}
	return 0, io.ErrUnexpectedEOF
}

func (b *errAfterBytesBody) Close() error { return nil }

// errAfterBytesRT 是 http.RoundTripper mock，让 /validate 返回带 50 字节
// remaining + io.ErrUnexpectedEOF 的 body。
type errAfterBytesRT struct {
	validateReadBytes *int32
	calls             *int32
}

func (rt *errAfterBytesRT) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(rt.calls, 1)
	if strings.Contains(req.URL.Path, "validate") &&
		!strings.Contains(req.URL.Path, "validateCaptcha") &&
		req.Method == http.MethodPost {
		body := &errAfterBytesBody{
			remaining: bytes.Repeat([]byte{'X'}, 50),
			readByAll: rt.validateReadBytes,
			firstRead: true,
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       body,
			Header:     http.Header{"Content-Length": []string{"50"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":1,"msg":"成功"}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

func (rt *errAfterBytesRT) Close() error { return nil }

// TestLogin_ReadAllError_ContainsStatusAndBytes 验证 Login 200 路径
// io.ReadAll 失败时，错误信息必须包含：
// - status code（这里是 200）
// - 已读字节数（这里是 50）
// 便于排查 server 端异常（连接 reset / content-length 不符）。
func TestLogin_ReadAllError_ContainsStatusAndBytes(t *testing.T) {
	var validateReadBytes int32
	var calls int32

	rt := &errAfterBytesRT{
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
	errStr := err.Error()

	// 必须 wrap io.ErrUnexpectedEOF
	if !errors.Is(err, io.ErrUnexpectedEOF) && !strings.Contains(errStr, io.ErrUnexpectedEOF.Error()) {
		t.Errorf("期望 wrap io.ErrUnexpectedEOF，实际: %v", err)
	}
	// G3 关键断言：错误必须包含 status code
	if !strings.Contains(errStr, "status=200") {
		t.Errorf("期望错误包含 'status=200' 上下文，实际: %v", err)
	}
	// G3 关键断言：错误必须包含已读字节数
	if !strings.Contains(errStr, "read=50") {
		t.Errorf("期望错误包含 'read=50' 上下文，实际: %v", err)
	}
}
