// Package client_test 外部包测试。
//
// F8: pkg/client/auth.go:368 syncCookieToken 静默 warn — 回归测试。
//
// 历史 bug：类型断言失败仅 Warn 不返回 error，WithHTTPClient 自定义 Jar
// （非 *cookiejar.Jar）时 X-Auth-Token 同步到 cookie 失败，业务接口返回空
// dataList 但根因在 build client 阶段的 stderr Warn，跨多步调用难关联。
//
// 修复后：
//   - syncCookieToken(token string) 改为 syncCookieToken(token string) error
//   - 类型断言失败时返回 error（用 fmt.Errorf 包装，引用 WithHTTPClient 文档提示）
//   - pkg/client/client.go:169-171 的 New() 末尾检查 error 并 propagate
//
// 验证策略：外部包 client_test 调用 client.New(WithHTTPClient(&http.Client{}),
// WithToken("x"))，断言 New() 返回 error 且 error 信息提示 Jar 类型问题。
package client_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestNew_WithHTTPClient_NonCookieJar_ReturnsError 验证当用户传入自定义
// http.Client（非默认 *cookiejar.Jar）且使用 WithToken 时，New() 必须返回
// error 让调用方立即感知，而不是只在 stderr Warn 静默吞掉。
//
// 修复前：syncCookieToken 只 Warn → 业务接口后续返回空 dataList
//
//	跨多步调用难关联根因。
//
// 修复后：syncCookieToken 返回 error → New() propagate → 调用方立即拿到 error。
func TestNew_WithHTTPClient_NonCookieJar_ReturnsError(t *testing.T) {
	// 自定义 http.Client：Jar 为 nil（不是 *cookiejar.Jar）
	customHTTP := &http.Client{Timeout: 15 * time.Second}

	// 调用 New + WithHTTPClient + WithToken，期待返回 error
	c, err := client.New(
		client.WithHTTPClient(customHTTP),
		client.WithToken("fake-token"),
	)

	if err == nil {
		t.Fatalf("New(WithHTTPClient(non-jar)+WithToken) 应返回 error，实际 nil。c=%v", c)
	}
	// Client 仍然返回（不为 nil），但带 error
	if c == nil {
		t.Fatal("New 返回 nil Client 但 error 非空，行为不一致")
	}
	// error 信息应提示 Jar 类型问题 + 修复方法
	errMsg := err.Error()
	if !strings.Contains(errMsg, "cookie") && !strings.Contains(errMsg, "Jar") {
		t.Errorf("error 信息应提示 cookie/Jar 问题，实际: %v", err)
	}
	// 验证提示用 client.New() 默认或显式 &http.Client{Jar: cookiejar.New(nil)}
	if !strings.Contains(errMsg, "cookiejar") && !strings.Contains(errMsg, "client.New") {
		t.Errorf("error 信息应提示修复方法（用 cookiejar.New() 或 client.New() 默认），实际: %v", err)
	}
}

// TestNew_DefaultClient_WithToken_NoError 验证默认 client.New() + WithToken
// 不会返回 error（默认 http.Client 有 cookiejar.Jar）。
func TestNew_DefaultClient_WithToken_NoError(t *testing.T) {
	c, err := client.New(
		client.WithToken("default-token"),
	)

	if err != nil {
		t.Fatalf("默认 Client + WithToken 应无 error，实际: %v", err)
	}
	if c == nil {
		t.Fatal("New 返回 nil")
	}
}

// TestNew_NoToken_NoError 验证无 WithToken 时永远不返回 error（baseline）。
func TestNew_NoToken_NoError(t *testing.T) {
	c, err := client.New()

	if err != nil {
		t.Fatalf("无 WithToken 时 New 应无 error，实际: %v", err)
	}
	if c == nil {
		t.Fatal("New 返回 nil")
	}
}

// TestNew_WithHTTPClient_NoToken_NoError 验证 WithHTTPClient 但无 WithToken
// 时不返回 error（无 cookie 同步需求，不触发断言失败）。
func TestNew_WithHTTPClient_NoToken_NoError(t *testing.T) {
	customHTTP := &http.Client{Timeout: 15 * time.Second}

	c, err := client.New(
		client.WithHTTPClient(customHTTP),
	)

	if err != nil {
		t.Fatalf("WithHTTPClient 但无 WithToken 应无 error，实际: %v", err)
	}
	if c == nil {
		t.Fatal("New 返回 nil")
	}
}
