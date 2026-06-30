// Package client 白盒测试：G2 doBizGet 非 200 响应包装 sentinel。
//
// 修复动机：doBizGet 收到非 200 时用 fmt.Errorf 裸返回，SDK 用户无法
// errors.Is 识别原因（限流 / 服务端异常 / HTTP 层错误），只能字符串匹配。
//
// 修复策略：按 StatusCode 切换 sentinel 包装：
//   - 429 → ErrRateLimited（限流，SDK 用户退避后重试）
//   - 5xx → ErrServiceUnavailable（服务端临时不可用）
//   - 其他 4xx → ErrInvalidResponse（HTTP 协议层错误，区别于业务 code=0）
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestG2_DoBizGet_429_RateLimited 验证 429 响应包装 ErrRateLimited。
func TestG2_DoBizGet_429_RateLimited(t *testing.T) {
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

	_, err := c.doBizGet(context.Background(), srv.URL+"/limited", nil)
	if err == nil {
		t.Fatal("429 应返回非 nil error")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("429 应包装 ErrRateLimited，实际: %v", err)
	}
}

// TestG2_DoBizGet_5xx_ServiceUnavailable 验证 5xx 响应包装 ErrServiceUnavailable。
func TestG2_DoBizGet_5xx_ServiceUnavailable(t *testing.T) {
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

			_, err := c.doBizGet(context.Background(), srv.URL+"/boom", nil)
			if err == nil {
				t.Fatalf("%d 应返回非 nil error", code)
			}
			if !errors.Is(err, ErrServiceUnavailable) {
				t.Errorf("%d 应包装 ErrServiceUnavailable，实际: %v", code, err)
			}
		})
	}
}

// TestG2_DoBizGet_4xxOther_InvalidResponse 验证 4xx 非 429 响应包装 ErrInvalidResponse。
func TestG2_DoBizGet_4xxOther_InvalidResponse(t *testing.T) {
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

			_, err := c.doBizGet(context.Background(), srv.URL+"/bad", nil)
			if err == nil {
				t.Fatalf("%d 应返回非 nil error", code)
			}
			if !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("%d 应包装 ErrInvalidResponse，实际: %v", code, err)
			}
		})
	}
}

// TestG2_DoBizGet_200_NoError 验证 200 响应不触发 sentinel（回归：非 200 包装不影响正常路径）。
func TestG2_DoBizGet_200_NoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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

	body, err := c.doBizGet(context.Background(), srv.URL+"/ok", nil)
	if err != nil {
		t.Errorf("200 不应返回 error，实际: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body 应为 'ok'，实际: %q", body)
	}
}
