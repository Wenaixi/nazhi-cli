// Package client 内部白盒测试。
package client

import (
	"io"
	"log/slog"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

// TestSyncCookieToken_OnlySetsCookieOnBaseURL 验证 syncCookieToken 只向 c.baseURL
// 域写入 X-Auth-Token，不向 c.ssoBaseURL 域写入。
//
// 历史 bug：auth.go:558 遍历 []string{c.ssoBaseURL, c.baseURL}，向两个域都写 cookie。
// SSO 域不需要 X-Auth-Token（它用 JSESSIONID），多余 cookie 无意义。
//
// 修复后：只写 c.baseURL 域。
func TestSyncCookieToken_OnlySetsCookieOnBaseURL(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com:8280",
		logger:     slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}
	c.http = newHTTPClient()
	c.http.Jar = jar

	if err := c.syncCookieToken("test-token"); err != nil {
		t.Fatalf("syncCookieToken 失败: %v", err)
	}

	ssoURL, _ := url.Parse("https://sso.example.com")
	bizURL, _ := url.Parse("https://biz.example.com:8280")

	// SSO 域不应有 X-Auth-Token
	ssoCookies := jar.Cookies(ssoURL)
	for _, ck := range ssoCookies {
		if ck.Name == "X-Auth-Token" {
			t.Errorf("SSO 域不应有 X-Auth-Token cookie，实际发现 value=%q", ck.Value)
		}
	}

	// baseURL 域应有 X-Auth-Token
	found := false
	bizCookies := jar.Cookies(bizURL)
	for _, ck := range bizCookies {
		if ck.Name == "X-Auth-Token" {
			found = true
			if ck.Value != "test-token" {
				t.Errorf("X-Auth-Token value 期望 %q，实际 %q", "test-token", ck.Value)
			}
		}
	}
	if !found {
		t.Error("baseURL 域应包含 X-Auth-Token cookie")
	}
}
