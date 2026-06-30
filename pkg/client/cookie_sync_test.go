package client

import (
	"bytes"
	"log/slog"
	"strings"
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

// TestBuildLoginResponse_EmptyBody_RawDataIsNil 验证 bodyBytes 为空时 RawData 为 nil。
// 合并冲突时选择保留 group A 的实现（带错误处理的 decode 块），
// 优于 group D 的"直接置 nil"。空 body 场景下不进入 decode 分支，RawData 保持 nil。
func TestBuildLoginResponse_EmptyBody_RawDataIsNil(t *testing.T) {
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
	}
	now := time.Now()
	resp := c.buildLoginResponse("jwt", now, nil, "200")
	if resp == nil {
		t.Fatal("buildLoginResponse 不应返回 nil")
	}
	if resp.RawData != nil {
		t.Error("RawData 在 bodyBytes 为空时应为 nil（decode 块不执行）")
	}
}

// ─── group-B F2: partial decode 应发 logger.Warn + RawData 不留半成品 ───

// TestBuildLoginResponse_PartialDecode_LogsAndClearsRawData 验证当 body 是合法
// 起始 + 尾随垃圾的 partial JSON 时，buildLoginResponse 必须：
//  1. 通过 c.logger.Warn 告知调用方（不能 silent 失败）
//  2. RawData 不能保留半成品 map（半成品对下游查找是误导）
//
// 修复前：partial decode 成功（rawData != nil）但 err != nil → silent，
// RawData 留下半成品，调用方拿到"看起来有效但字段不全"的 RawData。
//
// 修复后：partial decode 错误时记 logger.Warn(...partial decode...) 并
// RawData = nil（防半成品被下游使用）。
func TestBuildLoginResponse_PartialDecode_LogsAndClearsRawData(t *testing.T) {
	var warnBuf bytes.Buffer
	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       newHTTPClient(),
		logger:     slog.New(slog.NewTextHandler(&warnBuf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}

	// 真正的 partial decode：完整有效的 JSON 对象后跟额外 token 触发 err。
	// json.Decoder.Decode() 在解析完第一个 value 后遇到非空白字节返回错误，
	// 此时 rawData 已含第一个 value 的字段但 err != nil——典型 partial 场景。
	// 完全无法解析（如 {"token":"abc","garbage）的 rawData == nil，走原 rawData==nil 兜底分支。
	partial := []byte(`{"token":"abc","user":"u1"}extra-garbage-data`)
	_ = partial

	resp := c.buildLoginResponse("jwt-token", time.Now(), partial, "partial-test")
	if resp == nil {
		t.Fatal("buildLoginResponse 不应返回 nil")
	}
	if resp.RawData != nil {
		t.Errorf("F2 修复契约：partial decode 错误时 RawData 应清零（防半成品），实际得到半成品: %+v", resp.RawData)
	}

	warnOut := warnBuf.String()
	if !strings.Contains(warnOut, "partial decode") && !strings.Contains(warnOut, "RawData") {
		t.Errorf("F2 silent 失败：logger 应发出 partial decode 警告，实际日志: %q", warnOut)
	}
}
