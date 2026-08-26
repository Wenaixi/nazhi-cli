package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPDo_RejectsOversizedBody 锁定 HTTP-2 契约：
// httpDo 读响应体必须封顶（防异常/被劫持服务端塞超大 body 的内存放大），
// 超限归 ErrInvalidResponse 而非继续全量读入。
func TestHTTPDo_RejectsOversizedBody(t *testing.T) {
	// 构造 >1MB 响应体
	huge := strings.Repeat("a", 2<<20) // 2MB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, huge)
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.httpDo(context.Background(), "GET", srv.URL+"/api/x", nil, map[string]string{}, "")
	if err == nil {
		t.Fatal("超大响应体应报错，实际 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("超大响应体应归 ErrInvalidResponse，实际 %v", err)
	}
}
