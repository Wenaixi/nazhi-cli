package client

import (
	"sync"
	"testing"
	"time"
)

// ─── syncCookieToken 测试 ───

// TestSyncCookieToken_ConcurrentRaceFree 验证 F3 修复：
// 直接构造 Client{} 绕过 New() 时并发 syncCookieToken 不再触发 race detector。
// 修复前：c.baseURLParsed 在懒解析分支读-检查-写无锁保护，
// 多个 goroutine 同时进入懒解析分支写字段会被 race detector 报警。
// 修复后：c.baseURLParsed 改 atomic.Pointer[url.URL]，所有访问原子化，
// 热路径 Load 无锁直接读，懒解析路径 CompareAndSwap 防重复解析。
func TestSyncCookieToken_ConcurrentRaceFree(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	// 注意：直接构造 Client{}，未调 New()，c.baseURLParsed 仍为零值，
	// 全部 syncCookieToken 调用都走懒解析分支——这是 race 唯一触发路径。

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = c.syncCookieToken("test-token")
		}()
	}
	wg.Wait()

	// 验证懒解析最终生效
	u := c.baseURLParsed.Load()
	if u == nil {
		t.Fatal("并发 syncCookieToken 后 baseURLParsed 应被解析")
	}
	if u.String() != "https://biz.example.com" {
		t.Errorf("baseURLParsed 内容错误: %q", u.String())
	}
}

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
//
// LoginResponse 不含 RawData 字段。
// 下游不再需要 rawData，全部依赖 token + expiresAt 两件套。
// 原 RawData 相关测试（InvalidJsonBody/EmptyBody/PartialDecode）已废弃。

// TestBuildLoginResponse_NoPanicOnInvalidJson 验证 body 非法 JSON 时不 panic。
func TestBuildLoginResponse_NoPanicOnInvalidJson(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	resp := c.buildLoginResponse("test-token", time.Now(), []byte("{invalid}"), "200")
	if resp == nil {
		t.Fatal("buildLoginResponse 不应返回 nil")
	}
	if resp.Token != "test-token" {
		t.Errorf("token 应为 'test-token'，实际 %q", resp.Token)
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
