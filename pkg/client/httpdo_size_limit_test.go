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
// TestDoBizGet_RejectsOversizedBody 锁定 HTTP-2 契约在 doBizGet 的遗漏路径：
// doBizGet（激活步骤1/InitSession/验证码拉取的共用 helper）读响应体同样必须封顶，
// 超限归 ErrInvalidResponse 而非继续全量读入（P1-1，auth-session 域 19 轮审计）。
func TestDoBizGet_RejectsOversizedBody(t *testing.T) {
	// 构造 >4MB 响应体（与 TestHTTPDo_RejectsOversizedBody 同尺寸）
	huge := strings.Repeat("a", 5<<20) // 5MB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, huge)
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

	_, err := c.doBizGet(context.Background(), srv.URL+"/", nil)
	if err == nil {
		t.Fatal("超大响应体应报错，实际 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("超大响应体应归 ErrInvalidResponse，实际 %v", err)
	}
}

// TestHTTPDo_AcceptsUpTo4MB 锁定 2026-08-27 事故修复：公示全量 JSON 实测超 1MB，
// 旧上限 1MB 直接拒绝导致公示 Tab 永不加载。4MB 上限下 2MB 响应必须正常通过。
func TestHTTPDo_AcceptsUpTo4MB(t *testing.T) {
	huge := strings.Repeat("a", 2<<20) // 2MB < 4MB 上限
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
	if err != nil {
		t.Fatalf("2MB 响应体在上限 4MB 下应通过，实际 %v", err)
	}
}

func TestHTTPDo_RejectsOversizedBody(t *testing.T) {
	// 构造 >4MB 响应体
	huge := strings.Repeat("a", 5<<20) // 5MB > 4MB 上限
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
