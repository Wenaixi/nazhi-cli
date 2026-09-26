package client

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

// newCookieSyncClient 构造只用于 cookie 同步测试的最小 Client。
func newCookieSyncClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New 失败: %v", err)
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Jar: jar},
	}
}

// TestSyncCookieToken_RejectsUnsupportedScheme 锁定「cookie 未真正写入」不再静默报成功。
//
// 背景：net/http/cookiejar 的 SetCookies 对非 http/https 的 scheme 直接
// return（标准库行为，无返回值、无报错）。原实现在此之后无条件
// logDebug + return nil，于是无 scheme 的 baseURL（如 "example.com" 或
// "ftp://host"）下同步「报成功但 cookie 未落地」——所有业务接口随后返回空
// dataList，而排查方向被误导到服务端。ErrCookieSyncFailed 正是为这类
// 「配置侧 cookie 不可用」而设，此处不能反而从它手上漏过去。
func TestSyncCookieToken_RejectsUnsupportedScheme(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
	}{
		{"无 scheme", "example.com"},
		{"无 scheme 带路径", "example.com/api"},
		{"不支持的 scheme", "ftp://example.com"},
		{"自定义 scheme", "ws://example.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := newCookieSyncClient(t, c.baseURL)
			err := client.syncCookieToken("tok-abc")
			if err == nil {
				t.Fatalf("baseURL=%q 的 scheme 不可写入 cookie，应返回错误，实际 nil", c.baseURL)
			}
			if !errors.Is(err, ErrCookieSyncFailed) {
				t.Errorf("应 errors.Is(ErrCookieSyncFailed)，实际: %v", err)
			}
		})
	}
}

// TestSyncCookieToken_AcceptsHTTPAndHTTPS 确认合法 scheme 仍正常写入。
func TestSyncCookieToken_AcceptsHTTPAndHTTPS(t *testing.T) {
	for _, base := range []string{"http://example.com", "https://example.com"} {
		t.Run(base, func(t *testing.T) {
			client := newCookieSyncClient(t, base)
			if err := client.syncCookieToken("tok-abc"); err != nil {
				t.Fatalf("合法 baseURL %q 不应报错: %v", base, err)
			}
			u, err := url.Parse(base)
			if err != nil {
				t.Fatalf("url.Parse 失败: %v", err)
			}
			cookies := client.http.Jar.Cookies(u)
			found := false
			for _, ck := range cookies {
				if ck.Name == "X-Auth-Token" && ck.Value == "tok-abc" {
					found = true
				}
			}
			if !found {
				t.Errorf("cookie 应已写入 jar，实际 cookies=%v", cookies)
			}
		})
	}
}
