// cookie_test.go 聚合 sync_cookie_* 内部白盒测试（package client）：
//   - syncCookieToken 只写 baseURL 域
//   - syncCookieToken baseURL 畸形 propagate error
package client

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

// ─── sync_cookie_domain_test.go: 只写 baseURL 域 ───

// TestSyncCookieToken_OnlySetsCookieOnBaseURL 验证 syncCookieToken 只向 c.baseURL
// 域写入 X-Auth-Token，不向 c.ssoBaseURL 域写入。
// 历史 bug：auth.go:558 遍历 []string{c.ssoBaseURL, c.baseURL}，向两个域都写 cookie。
// SSO 域不需要 X-Auth-Token（它用 JSESSIONID），多余 cookie 无意义。
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

// ─── sync_cookie_url_error_test.go (F5): baseURL 畸形 propagate ───

// malformedBaseURL 是会让 url.Parse 失败的固定输入：含 DEL (0x7f) 控制字符。
// 用 const 保证断言能精确匹配 error 中的 raw 字符串。
const malformedBaseURL = "http://example.com\x7f with space"

// newTestClientWithJar 构造一个最小可用 *Client 用于 syncCookieToken 白盒测试。
// logger 必填（slog 默认 nil 会在 logDebug 时 panic），Jar 用 cookiejar.New。
func newTestClientWithJar(ssoBase, bizBase string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		ssoBaseURL: ssoBase,
		baseURL:    bizBase,
		http:       &http.Client{Jar: jar},
		logger:     slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}
}

// TestSyncCookieToken_BaseURLMalformed_Propagates 验证 baseURL 畸形时
// syncCookieToken 返回 error（修复后只解析 c.baseURL，ssoBaseURL 不参与 cookie 写入）。
func TestSyncCookieToken_BaseURLMalformed_Propagates(t *testing.T) {
	c := newTestClientWithJar("https://sso.example.com", malformedBaseURL)

	err := c.syncCookieToken("test-token")
	if err == nil {
		t.Fatal("base URL 畸形时应 propagate error，实际 nil")
	}
	if !strings.Contains(err.Error(), "syncCookieToken") {
		t.Errorf("error 应包含 syncCookieToken 前缀，实际: %v", err)
	}
}

// TestSyncCookieToken_AllBaseURLsValid_NoError 验证 base URL 合法时
// syncCookieToken 成功（happy path）。
func TestSyncCookieToken_AllBaseURLsValid_NoError(t *testing.T) {
	c := newTestClientWithJar("https://sso.example.com", "https://biz.example.com")

	err := c.syncCookieToken("test-token")
	if err != nil {
		t.Fatalf("所有 base URL 合法时不应返回 error，实际: %v", err)
	}
}
