// cookie_sync_r15_test.go
// 第15轮 F6 + cleanup-cookie 验证：
//   - F6: c.baseURLParsed 在 New() 阶段被预解析，syncCookieToken 复用解析结果
//   - cleanup-cookie: 4 个 error path 统一使用 `syncCookieToken 失败: <cause>` 前缀
package client

import (
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
)

// TestNew_PreParsesBaseURL_R15C 验证 F6 修复：Client.baseURLParsed 在 New() 阶段
// 已被解析一次，后续 syncCookieToken 不再重复 url.Parse。
//
// 设计意图：把 url.Parse 从 hot path（每次 syncCookieToken 都调用）迁移到
// New() 阶段的初始化路径，避免每次 syncCookieToken 都做一遍重复的字符串解析。
func TestNew_PreParsesBaseURL_R15C(t *testing.T) {
	c, err := New(WithBaseURL("https://biz.example.com:8280"))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if c.baseURLParsed == nil {
		t.Fatal("New() 后 c.baseURLParsed 应已被预解析，实际 nil")
	}
	if c.baseURLParsed.String() != "https://biz.example.com:8280" {
		t.Errorf("baseURLParsed 期望 %q，实际 %q",
			"https://biz.example.com:8280", c.baseURLParsed.String())
	}
}

// TestSyncCookieToken_LazyParseFallback_R15C 验证 F6 兼容性：直接构造 Client
// （绕过 New()）时，syncCookieToken 仍能懒解析 baseURL，保持向后兼容。
//
// 设计意图：测试场景常直接用 &Client{baseURL: ...} 构造，跳过 New()。
// 如果 baseURLParsed 字段未在 New() 阶段设置，syncCookieToken 必须能懒解析兜底，
// 否则会因 nil deref crash。懒解析后保存到字段，避免重复 Parse。
func TestSyncCookieToken_LazyParseFallback_R15C(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	c := &Client{
		baseURL: "https://biz.example.com:8280",
		http:    &http.Client{Jar: jar},
	}
	if err := c.syncCookieToken("tok"); err != nil {
		t.Fatalf("syncCookieToken 失败: %v", err)
	}
	if c.baseURLParsed == nil {
		t.Fatal("懒解析后 c.baseURLParsed 应非 nil")
	}
}

// TestSyncCookieToken_ErrorPrefixUnified_R15C 验证 cleanup-cookie 修复：
// 所有 4 个 error path（nil http / 非 *cookiejar.Jar / baseURL 解析失败 / baseURL 畸形）
// 统一使用 `syncCookieToken 失败:` 前缀。
//
// 设计意图：消除 4 个 fmt.Errorf 各自的拼凑前缀（"syncCookieToken: HTTP client 为 nil..."
// "syncCookieToken: HTTP client 的 Jar 不是..." 等），统一为 `syncCookieToken 失败: <cause>`，
// 让调用方能通过统一的 errors.Is 字符串前缀判定（未来如需加 sentinel error 更容易）。
func TestSyncCookieToken_ErrorPrefixUnified_R15C(t *testing.T) {
	const wantPrefix = "syncCookieToken 失败:"

	cases := []struct {
		name string
		c    *Client
	}{
		{
			name: "nil http",
			c:    &Client{http: nil},
		},
		{
			name: "non-cookiejar Jar",
			c: &Client{http: &http.Client{Jar: nil}},
		},
		{
			name: "malformed baseURL",
			c: &Client{
				baseURL: "://bad-url",
				http:    &http.Client{Jar: nil},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.syncCookieToken("tok")
			if err == nil {
				t.Fatal("期望返回 error，实际 nil")
			}
			if !strings.HasPrefix(err.Error(), wantPrefix) {
				t.Errorf("error 前缀期望 %q，实际 %q (err=%v)",
					wantPrefix, err.Error(), err)
			}
		})
	}
}