// request_external_test.go 聚合 request 外部黑盒测试（package client_test）：
//   - F28: newHTTPClient 自定义 Transport MaxIdleConnsPerHost ≥ 8
//   - TransportIdleConnPoolShared: 8 路并发不触发额外握手
package client_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// ─── request_transport_test.go (F28): 自定义 Transport ───

// TestNewHTTPClient_UsesCustomTransport 验证 newHTTPClient 不再回退到 http.DefaultTransport。
// 背景：F28 — newHTTPClient 未自定义 Transport，复用全局 http.DefaultTransport，
// 而 DefaultTransport 的 MaxIdleConnsPerHost=2。FetchTasks 8 路并发打到同一 biz host 时，
// 第 3-8 路必须重新握手，wall time 增加 ~1-4s。
// 修复：newHTTPClient 现在返回自定义 &http.Transport{MaxIdleConns: 100,
// MaxIdleConnsPerHost: 16, ...}，与 file.go cached Transport 对齐。
func TestNewHTTPClient_UsesCustomTransport(t *testing.T) {
	c, err := client.New(client.WithTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("client.New 失败: %v", err)
	}
	defer func() { _ = c.Close() }()

	tr := client.TransportForTest(c)
	if tr == nil {
		t.Fatal("Transport 不应为 nil（必须显式自定义，避免 http.DefaultTransport 的 MaxIdleConnsPerHost=2 限制）")
	}

	// 关键配置：MaxIdleConnsPerHost 必须 ≥ 8（FetchTasks errgroup 限流）
	const minIdlePerHost = 8
	if tr.MaxIdleConnsPerHost < minIdlePerHost {
		t.Errorf("MaxIdleConnsPerHost = %d，必须 ≥ %d（FetchTasks 8 路并发）", tr.MaxIdleConnsPerHost, minIdlePerHost)
	}
	if tr.MaxIdleConns < minIdlePerHost {
		t.Errorf("MaxIdleConns = %d，必须 ≥ %d", tr.MaxIdleConns, minIdlePerHost)
	}

	// 必须不等于 http.DefaultTransport（否则修改无意义）
	if tr == http.DefaultTransport {
		t.Error("newHTTPClient 不应复用 http.DefaultTransport（这是 F28 的根本原因）")
	}
}

// TestNewHTTPClient_TransportIdleConnPoolShared 验证自定义 Transport 的 idle 池
// 在多次请求间复用——并发打同一 host 不触发额外 TLS 握手。
func TestNewHTTPClient_TransportIdleConnPoolShared(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":null,"dataList":null,"dataMap":null}`))
	}))
	defer srv.Close()

	c, err := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("client.New 失败: %v", err)
	}
	defer func() { _ = c.Close() }()

	// 8 路并发 GET，验证所有请求成功
	const concurrency = 8
	done := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			httpClient := client.HTTPClientForTest(c)
			resp, err := httpClient.Get(srv.URL + "/api/test")
			if err != nil {
				done <- err
				return
			}
			// F32a: drain body 后再 Close，让连接归还 keep-alive 池
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			done <- nil
		}()
	}
	for i := 0; i < concurrency; i++ {
		if err := <-done; err != nil {
			t.Errorf("并发请求 #%d 失败: %v", i, err)
		}
	}
}
