// Package client 白盒测试：C1 doBizGet 非 200 响应 body 行为。
//
// Finding: doBizGet 在非 200 路径同时返回非 nil bodyBytes 和 error，违反 Go 约定。
// 正确行为：error 非 nil 时 bodyBytes 应为 nil。
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestC1_DoBizGet_Non200_ReturnsNilBody 验证：服务器返回 500 时，调用方收到 (nil, err) 而非 (body, err)。
func TestC1_DoBizGet_Non200_ReturnsNilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("<html>500 Internal Server Error</html>"))
	}))
	defer srv.Close()

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     nil,
		ocr:        nil,
	}

	bodyBytes, err := c.doBizGet(context.Background(), srv.URL+"/boom", nil)
	// 验证 error 非 nil
	if err == nil {
		t.Fatal("500 应返回非 nil error")
	}
	// 验证 bodyBytes 为 nil（符合 Go 约定：error 非 nil 时 body 也应为 nil）
	if bodyBytes != nil {
		t.Errorf("error 非 nil 时 bodyBytes 应为 nil，实际 bodyBytes=%q", string(bodyBytes))
	}
}
