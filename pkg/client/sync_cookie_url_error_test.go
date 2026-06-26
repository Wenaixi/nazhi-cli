// Package client 内部白盒测试。
//
// F5 (review-tdd 三轮): pkg/client/auth.go syncCookieToken baseURL 解析失败
// propagate error — 回归测试。
//
// 历史 bug：F8 round1 修了 Jar 类型断言失败 propagate error，但 baseURL 解析
// 失败仍静默 c.logger.Warn + continue + return nil，invariant 不对称：
//   - 类型断言失败 → 返回 error（caller 可在 build 阶段感知）
//   - URL 解析失败 → 静默 Warn + return nil（caller 收不到任何信号）
//
// 修复后：URL 解析失败也 propagate error（fmt.Errorf wrap），与 Jar 类型断言
// 失败的契约对齐。warnSyncCookieToken helper 继续 WARN 不阻断（业务 token
// 仍有效），但 New() 路径下 baseURL 由用户控制，错误应当 surface。
//
// 验证策略：白盒测试直接构造 *Client（含 slog.Logger 防 happy path 走
// logDebug 时 nil deref），ssoBaseURL/baseURL 设置为 url.Parse 失败的
// 畸形字符串（ASCII 控制字符），断言 syncCookieToken 返回 error。
package client

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
)

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
// syncCookieToken 返回 error（B9 后只解析 c.baseURL，ssoBaseURL 不参与 cookie 写入）。
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
