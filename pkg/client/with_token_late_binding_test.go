// Package client 内部白盒测试。
package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWithToken_LateBinding 回归测试：WithToken + WithSSOBase 顺序敏感性 bug。
// 历史 bug：WithToken 立即调 syncCookieToken 写 cookie 到当时的 c.ssoBaseURL，
// 若用户按 New(WithToken(t), WithSSOBase(u)) 顺序调用，token 写到 default
// SSO host 而非用户指定的 host，业务请求 0 cookie → 空数据。
// 修复后：WithToken 仅存到 c.pendingToken，New() 跑完所有 Options 后
// 才统一 syncCookieToken（此时 c.http.Jar / c.ssoBaseURL / c.baseURL
// 都是最终值）。
func TestWithToken_LateBinding(t *testing.T) {
	// mock 目标平台：记录收到的所有 X-Auth-Token cookie
	var receivedAuthToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("X-Auth-Token"); err == nil {
			receivedAuthToken = c.Value
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 关键：WithToken 在 WithSSOBase 之前调用（之前会写错 host）
	c, _ := New(
		WithToken("jwt-late-bind"),
		WithSSOBase(srv.URL),
		WithBaseURL(srv.URL),
	)

	// 触发实际请求，jar 应自动注入 X-Auth-Token cookie
	resp, err := c.http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http.Get failed: %v", err)
	}
	drainAndClose(resp.Body)

	if receivedAuthToken != "jwt-late-bind" {
		t.Errorf("请求未携带 X-Auth-Token=jwt-late-bind（实际收到 %q），WithToken cookie 写到了错误的 host",
			receivedAuthToken)
	}
}
