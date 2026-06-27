// request_test.go 聚合 request.go 内部白盒测试（package client）：
//   - F6: httpDo/doBizGet drain+close 让 keep-alive 池复用
//   - F1: logRequestHeaders nil logger 安全
//   - F-REDACT: logDebug 不泄漏完整 token（Referer/Cookie 嵌入场景）
//   - drainAndClose helper 单元测试
package client

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ─── drain_helper_test.go: drainAndClose helper 单元测试 ───

// fakeReadCloser 模拟 http.Response.Body：
// - Read() 从 data 读取
// - Close() 增加计数器
// - drained 用于检测 drainAndClose 真的读了所有数据
type fakeReadCloser struct {
	data     io.Reader
	closeCnt int
	drained  bool // 记录是否被 io.Copy 完整读到底
}

func (f *fakeReadCloser) Read(p []byte) (int, error) {
	n, err := f.data.Read(p)
	if err == io.EOF {
		f.drained = true
	}
	return n, err
}

func (f *fakeReadCloser) Close() error {
	f.closeCnt++
	return nil
}

// 真实场景测试：drainAndClose 必须先 drain body 再 Close()，
// 让 net/http 把连接归还 keep-alive 池（F1 修复的同类 bug 防御）。
func TestDrainAndClose_DrainsBeforeClose(t *testing.T) {
	body := &fakeReadCloser{data: strings.NewReader("hello world")}
	drainAndClose(body)

	if body.closeCnt != 1 {
		t.Errorf("Close 应被调用 1 次，实际 %d", body.closeCnt)
	}
	if !body.drained {
		t.Error("drainAndClose 应完整读完 body 后再 Close（drained=true）")
	}
}

// nil 安全：某些边缘路径（如响应 body 为 nil）不应 panic。
func TestDrainAndClose_NilSafe(t *testing.T) {
	drainAndClose(nil) // 不应 panic
}

// http.NoBody 等 ReadCloser 实现也必须正确 drain+close。
func TestDrainAndClose_NoBody(t *testing.T) {
	body := http.NoBody
	drainAndClose(body) // http.NoBody 的 Close 返回 nil，drain 是空操作
}

// ─── request_drain_test.go (F6): httpDo/doBizGet drain+close ───

// TestHttpDo_DrainsAndClosesForKeepAlive 回归测试：httpDo 的 defer
// 路径必须先 drain response body 再 Close，让 net/http 把连接归还 keep-alive 池。
func TestHttpDo_DrainsAndClosesForKeepAlive(t *testing.T) {
	var (
		mu      sync.Mutex
		idleHit int
	)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 关键：body 必须 > 4KB 以确保 ReadAll 不会一次性吞完，
		// 才能验证 defer 路径真的在 ReadAll 之后还能 drain 剩余字节。
		w.Header().Set("Content-Length", "8192")
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 64)
		for i := range chunk {
			chunk[i] = 'A'
		}
		// 重复写 128 次 = 8192 字节
		for i := 0; i < 128; i++ {
			_, _ = w.Write(chunk)
		}
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateIdle {
			mu.Lock()
			idleHit++
			mu.Unlock()
		}
	}
	srv.Start()
	defer srv.Close()

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        nil,
	}

	for i := 0; i < 2; i++ {
		body, err := c.httpDo(context.Background(), http.MethodGet, srv.URL+"/x", nil, nil, "")
		if err != nil {
			t.Fatalf("第 %d 次 httpDo 失败: %v", i+1, err)
		}
		if len(body) != 8192 {
			t.Errorf("第 %d 次 body 长度 = %d, want 8192", i+1, len(body))
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if idleHit == 0 {
		t.Errorf("httpDo defer 路径未把连接归还 keep-alive 池（idleHit=0）。\n" +
			"说明 defer 没有 drain+close，被 net/http 强制关闭了 TCP 连接。")
	}
}

// TestDoBizGet_DrainsAndClosesForKeepAlive 同上，验证 doBizGet 路径。
func TestDoBizGet_DrainsAndClosesForKeepAlive(t *testing.T) {
	var (
		mu      sync.Mutex
		idleHit int
	)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "8192")
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 64)
		for i := range chunk {
			chunk[i] = 'B'
		}
		for i := 0; i < 128; i++ {
			_, _ = w.Write(chunk)
		}
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateIdle {
			mu.Lock()
			idleHit++
			mu.Unlock()
		}
	}
	srv.Start()
	defer srv.Close()

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        nil,
	}

	for i := 0; i < 2; i++ {
		body, err := c.doBizGet(context.Background(), srv.URL+"/y", nil)
		if err != nil {
			t.Fatalf("第 %d 次 doBizGet 失败: %v", i+1, err)
		}
		if len(body) != 8192 {
			t.Errorf("第 %d 次 body 长度 = %d, want 8192", i+1, len(body))
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if idleHit == 0 {
		t.Errorf("doBizGet defer 路径未把连接归还 keep-alive 池（idleHit=0）。\n" +
			"说明 defer 没有 drain+close，被 net/http 强制关闭了 TCP 连接。")
	}
}

// ─── request_log_headers_test.go (F1): logRequestHeaders nil 安全 ───

// TestLogRequestHeaders_NilLogger_NoPanic 回归测试（F1）：
// logRequestHeaders 应当先检查 c.logger == nil，防止 nil pointer panic。
// 与 logDebug 的 nil 守卫（client.go:347）对称一致。
func TestLogRequestHeaders_NilLogger_NoPanic(t *testing.T) {
	c := &Client{logger: nil}
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 未修复前：c.logger.Enabled(...) 因 c.logger==nil 而 panic
	// 修复后应正常返回
	c.logRequestHeaders(req)
}

// ─── request_log_redact_test.go: token 不泄漏到日志 ───

// TestRequest_NoTokenLeakInDebugLog 回归测试：logDebug 不得把完整 token
// 写入日志，包括嵌入 Referer / Cookie / Authorization 等 header 的 token。
// 历史 bug：logDebug 只对 X-Auth-Token 截断到 16 字符，其他 header 完整打印。
// 但 session.go:37 会把完整 token 注入 Referer（如 `/homepage?token=<full-jwt>`），
// 触发完整 JWT 落 stderr → 凭据泄漏。
func TestRequest_NoTokenLeakInDebugLog(t *testing.T) {
	// 构造一个明显长于 16 字符的可识别 token
	const fullToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJURVNUMjAyNTAwMSIsImV4cCI6OTk5OTk5OTk5OX0.abcdefghijklmnop"

	// 捕获 slog Debug 输出
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 模拟目标平台：返回 200 + 空 body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// 白盒构造 Client（避免触发 OCR/SSL 等无关初始化）
	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        nil,
	}

	// 模拟 session.go:37 行为：把完整 token 注入 Referer query string
	headers := map[string]string{
		"X-Auth-Token": fullToken,
		"Referer":      srv.URL + "/homepage?token=" + fullToken,
		"Cookie":       "JSESSIONID=abc; X-Auth-Token=" + fullToken,
	}

	// 触发 httpDo 的 logDebug 路径（响应内容无关紧要）
	_, _ = c.httpDo(context.Background(), http.MethodGet, srv.URL+"/test", nil, headers, "")

	logs := logBuf.String()
	if strings.Contains(logs, fullToken) {
		t.Errorf("完整 token 泄漏到日志中（应被脱敏）：\n--- LOGS ---\n%s--- END ---", logs)
	}
}
