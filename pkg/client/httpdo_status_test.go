// Package client 白盒测试：httpDo 非 2xx 走 classifyHTTPStatus。
//
// 修复动机：httpDo 原先只 ReadAll 返回 body,nil，完全忽略 StatusCode；
// doBizAndDecode 主路径因此无法识别 401/429/5xx，只能在 JSON 层误报解析/业务错误。
//
// 修复策略：httpDo 对非 2xx 调用 classifyHTTPStatus 返回 sentinel，
// 与 doBizGet 行为对齐（429→ErrRateLimited / 5xx→ErrServiceUnavailable / 其他→ErrInvalidResponse）。
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHttpDo_429_RateLimited 验证 httpDo 对 429 包装 ErrRateLimited。
func TestHttpDo_429_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limit hit"))
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

	_, err := c.httpDo(context.Background(), http.MethodGet, srv.URL+"/limited", nil, nil, "")
	if err == nil {
		t.Fatal("429 应返回非 nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("429 应包装 ErrRateLimited，实际: %v", err)
	}
}

// TestHttpDo_5xx_ServiceUnavailable 验证 httpDo 对 5xx 包装 ErrServiceUnavailable。
func TestHttpDo_5xx_ServiceUnavailable(t *testing.T) {
	cases := []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout}
	for _, code := range cases {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte("server boom"))
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

			_, err := c.httpDo(context.Background(), http.MethodPost, srv.URL+"/boom", map[string]string{"k": "v"}, nil, "")
			if err == nil {
				t.Fatalf("%d 应返回非 nil error", code)
			}
			if !errors.Is(err, ErrServiceUnavailable) {
				t.Errorf("%d 应包装 ErrServiceUnavailable，实际: %v", code, err)
			}
		})
	}
}

// TestHttpDo_4xxOther_InvalidResponse 验证 httpDo 对 4xx 非 429 包装 ErrInvalidResponse。
func TestHttpDo_4xxOther_InvalidResponse(t *testing.T) {
	cases := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound}
	for _, code := range cases {
		t.Run(fmt.Sprintf("code_%d", code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte("client error"))
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

			_, err := c.httpDo(context.Background(), http.MethodGet, srv.URL+"/bad", nil, nil, "")
			if err == nil {
				t.Fatalf("%d 应返回非 nil error", code)
			}
			if !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("%d 应包装 ErrInvalidResponse，实际: %v", code, err)
			}
		})
	}
}

// TestHttpDo_200_NoError 验证 200 不触发 sentinel（回归）。
func TestHttpDo_200_NoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1}`))
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

	body, err := c.httpDo(context.Background(), http.MethodGet, srv.URL+"/ok", nil, nil, "")
	if err != nil {
		t.Fatalf("200 不应返回 error，实际: %v", err)
	}
	if string(body) != `{"code":1}` {
		t.Errorf("body 应为 JSON，实际: %q", body)
	}
}

// TestDoBizAndDecode_429_RateLimited 验证 doBizAndDecode 主路径经 httpDo 能命中 ErrRateLimited，
// 而不是把 429 的 HTML/空 body 当成 JSON 解析错误。
func TestDoBizAndDecode_429_RateLimited(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limit"))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("429 应返回非 nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("doBizAndDecode 对 429 应包装 ErrRateLimited，实际: %v", err)
	}
}

// TestDoBizAndDecode_401_InvalidResponse 验证 doBizAndDecode 对 401 命中 ErrInvalidResponse。
func TestDoBizAndDecode_401_InvalidResponse(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodPost, map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("401 应返回非 nil error")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("doBizAndDecode 对 401 应包装 ErrInvalidResponse，实际: %v", err)
	}
}

// TestDoBizAndDecode_503_ServiceUnavailable 验证 doBizAndDecode 对 503 命中 ErrServiceUnavailable。
func TestDoBizAndDecode_503_ServiceUnavailable(t *testing.T) {
	biz := httptest.NewServer(testBizHandlerDoBiz(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("service down"))
	}))
	defer biz.Close()

	c, err := New(WithBaseURL(biz.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.doBizAndDecode(context.Background(), "test-token", "TestOp", "/api/test", http.MethodGet, nil)
	if err == nil {
		t.Fatal("503 应返回非 nil error")
	}
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("doBizAndDecode 对 503 应包装 ErrServiceUnavailable，实际: %v", err)
	}
}
