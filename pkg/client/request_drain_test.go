// Package client 内部白盒测试。
package client

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestDoRequest_DrainsAndClosesForKeepAlive 回归测试：doRequest 的 defer
// 路径必须先 drain response body 再 Close，让 net/http 把连接归还 keep-alive 池。
// 历史背景：request.go:145 inline 的 `defer func(){io.Copy+Close}()` 是
// F6 已确立的 drainAndClose helper 的同款代码——
// 本测试防止有人把代码改回 verbatim inline（f1 修复的同类回归防御）。
// 验证策略：监听 httptest.Server 的 ConnState，断言两个连续请求
// 之后连接经历过 StateIdle（即被归还到 keep-alive 池供复用）。
// 如果 defer 没有 drain 就 close，net/http 会强制关闭 TCP，
// 永远看不到 StateIdle。
func TestDoRequest_DrainsAndClosesForKeepAlive(t *testing.T) {
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
	// ConnState 必须在 Start() 之前设置，否则 httptest.Server 已开始接受连接，
	// 服务器 goroutine 读 ConnState 的同时测试 goroutine 写 → data race。
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

	// 触发 2 次 doRequest，验证 keep-alive 复用
	for i := 0; i < 2; i++ {
		body, err := c.doRequest(context.Background(), http.MethodGet, srv.URL+"/x", nil, nil, "")
		if err != nil {
			t.Fatalf("第 %d 次 doRequest 失败: %v", i+1, err)
		}
		if len(body) != 8192 {
			t.Errorf("第 %d 次 body 长度 = %d, want 8192", i+1, len(body))
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if idleHit == 0 {
		t.Errorf("doRequest defer 路径未把连接归还 keep-alive 池（idleHit=0）。\n" +
			"说明 defer 没有 drain+close，被 net/http 强制关闭了 TCP 连接。")
	}
}

// TestDoBizGet_DrainsAndClosesForKeepAlive 同上，验证 doBizGet 路径。
// 修复要点：request.go:193-197 inline 的 verbatim defer
// 必须替换为同文件 drainAndClose helper，与 doRequest 保持对称。
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
