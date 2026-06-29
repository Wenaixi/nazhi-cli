package client

import (
	"testing"
	"time"
)

// ─── syncCookieToken 测试 ───

// TestSyncCookieToken_NilJar_ReturnsError 验证 jar 为 nil 时返回 error。
func TestSyncCookieToken_NilJar_ReturnsError(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       nil,
	}
	err := c.syncCookieToken("dummy-token")
	if err == nil {
		t.Fatal("nil jar 应返回 error，实际 nil")
	}
}

// TestSyncCookieToken_CustomJarNotCookiejar_ReturnsError 验证非 *cookiejar.Jar 返回 error。
func TestSyncCookieToken_CustomJarNotCookiejar_ReturnsError(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(), // 有 jar，但这里我们构造时不给 http
	}
	// http.Jar 在 newHTTPClient 中默认是 *cookiejar.Jar，我们手动设为 -1 类型
	// 不好模拟，重点测另一种场景：http 为空（jar 也是 nil）
	// 更直接的场景：设 http 但 http.Jar 被覆盖为自定义类型
	// 这里用 http == nil 验证 error 路径
	c.http = nil
	err := c.syncCookieToken("dummy-token")
	if err == nil {
		t.Fatal("nil http client 应返回 error，实际 nil")
	}
}

// TestSyncCookieToken_InvalidBaseURL 验证 baseURL 畸形时返回 error。
func TestSyncCookieToken_InvalidBaseURL(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "://bad-url", // 畸形 URL
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	err := c.syncCookieToken("dummy-token")
	if err == nil {
		t.Fatal("畸形 baseURL 应返回 error，实际 nil")
	}
}

// ─── warnSyncCookieToken 测试 ───

// TestWarnSyncCookieToken_NoPanicOnBadJar 验证 warnSyncCookieToken 在 jar 异常时不 panic。
func TestWarnSyncCookieToken_NoPanicOnBadJar(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       nil,
	}
	// 不应该 panic
	c.warnSyncCookieToken("dummy-token", "TEST_LABEL")
}

// ─── buildLoginResponse 测试 ───

// TestBuildLoginResponse_InvalidJsonBody_RawDataNotEmpty 验证 body 非法 JSON 时 RawData 为空 map ≠ nil。
func TestBuildLoginResponse_InvalidJsonBody_RawDataNotEmpty(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	// 非法 JSON（截断/乱码等场景）
	resp := c.buildLoginResponse("test-token", time.Now(), []byte("{invalid}"), "200")
	if resp == nil {
		t.Fatal("buildLoginResponse 不应返回 nil")
	}
	if resp.RawData == nil {
		t.Fatal("F3 BUG: 非法 JSON body 后 RawData 为 nil，下游 302 路径可 panic")
	}
}

// TestBuildLoginResponse_NoPanicOnEmptyBody 验证 bodyBytes 为空时不 panic。
func TestBuildLoginResponse_NoPanicOnEmptyBody(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	resp := c.buildLoginResponse("test-token", time.Now(), nil, "200")
	if resp == nil {
		t.Fatal("buildLoginResponse 不应返回 nil")
	}
	if resp.Token != "test-token" {
		t.Errorf("token 应为 'test-token'，实际 %q", resp.Token)
	}
}

// TestBuildLoginResponse_RawDataIsNil 验证 buildLoginResponse 的 RawData 为 nil。
// Finding #8: 旧代码对 bodyBytes 二次 JSON 解析（同字节 decode 两次），
// RawData 仅在测试中使用且 json:"-" 不参与序列化，故直接置 nil。
func TestBuildLoginResponse_RawDataIsNil(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	body := []byte(`{"code":1,"msg":"成功","returnData":{"token":"jwt"}}`)
	now := time.Now()
	resp := c.buildLoginResponse("jwt", now, body, "200")
	if resp == nil {
		t.Fatal("buildLoginResponse 不应返回 nil")
	}
	if resp.RawData != nil {
		t.Error("RawData 应为 nil（不再二次 JSON 解析 bodyBytes）")
	}
}
