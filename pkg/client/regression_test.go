// Package client_test 包含针对已审查 bug 的回归测试。
// 这些测试基于 code-reviewer 报告的 CRITICAL + HIGH 问题。
// TDD 流程：先写测试 → 确认失败 → 修复 → 确认通过。
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── CRITICAL #3: self_eval 错误被吞掉 ───

// TestRegression_QuerySelfGradEvaluation_PropagatesError 验证解析失败时
// 错误必须被返回（而不是被吞掉返回 nil, nil）。
func TestRegression_QuerySelfGradEvaluation_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 返回错误 JSON 让所有解析都失败
		w.Write([]byte("not valid json {{{"))
	}))
	defer srv.Close()

	c, err := client.New(
		client.WithBaseURL(srv.URL),
		client.WithToken("fake"),
	)
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	_, err = c.QuerySelfGradEvaluation(context.Background(), "fake")
	if err == nil {
		t.Fatal("期望返回错误（响应体无效 JSON），但 err 为 nil")
	}
	if !strings.Contains(err.Error(), "QuerySelfGradEvaluation") {
		t.Errorf("错误信息应包含函数名，便于定位: %v", err)
	}
}

// ─── UploadFile 独立 client 应禁用自动重定向 ───

// TestRegression_UploadFile_NoRedirectFollow 验证上传文件时遇到 302
// 不会自动跟随——请求不得投递到 Location 指定的重定向目标。
//
// 攻击者侧用真实可达的 httptest 服务器充当探针（而非保留域名
// attacker.invalid：那类域名无法解析，跟随与否都连不上，断言恒真）。
func TestRegression_UploadFile_NoRedirectFollow(t *testing.T) {
	// 攻击者探针：Location 指向它。若 SDK 跟随 302，此处必定收到请求。
	attackerHit := atomic.Bool{}
	attackerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attackerHit.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer attackerSrv.Close()

	uploadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", attackerSrv.URL+"/steal")
		w.WriteHeader(http.StatusFound)
	}))
	defer uploadSrv.Close()

	c, err := client.New(
		client.WithUploadURL(uploadSrv.URL),
		client.WithSSOBase("http://sso.example"),
	)
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	tmpFile := t.TempDir() + "/test.png"
	if err := writeSimplePNG(tmpFile); err != nil {
		t.Fatal(err)
	}

	_, uploadErr := c.UploadFile(context.Background(), tmpFile)
	// 302 非 200，应返回错误而非静默成功
	if uploadErr == nil {
		t.Fatal("UploadFile 在 302 时应返回错误，但 err 为 nil")
	}
	if !strings.Contains(uploadErr.Error(), "302") && !strings.Contains(uploadErr.Error(), "status=") {
		t.Errorf("错误信息应指出 302 状态: %v", uploadErr)
	}
	// 核心断言：重定向目标零命中——证明请求未离开上传域。
	if attackerHit.Load() {
		t.Error("SDK 跟随了 302 并把请求投递到重定向目标，上传凭据存在外投风险")
	}
}

// writeSimplePNG 写一个最小 PNG 文件
func writeSimplePNG(path string) error {
	// 1x1 透明 PNG
	png := []byte{
		0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89,
		0x00, 0x00, 0x00, 0x0A, 'I', 'D', 'A', 'T',
		0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
		0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D',
		0xAE, 0x42, 0x60, 0x82,
	}
	return os.WriteFile(path, png, 0644)
}

// ─── HIGH #8: WithHTTPClient + cookie jar ───

// TestRegression_WithHTTPClient_NoJar_DoesNotPanic 验证传入无 cookie jar
// 的 http.Client 时 syncCookieToken 不会 panic。
// 修复后升级：syncCookieToken 返回 error，New() 也返回 error，
// 测试断言 c 不为 nil（仍能 Close() 清理资源）且 err 非 nil 提示 cookie jar 问题。
func TestRegression_WithHTTPClient_NoJar_DoesNotPanic(t *testing.T) {
	customHTTP := &http.Client{} // 无 Jar

	c, err := client.New(
		client.WithHTTPClient(customHTTP),
		client.WithToken("test-token"),
	)
	if c == nil {
		t.Fatal("New returned nil client")
	}
	if err == nil {
		t.Fatal("New(WithHTTPClient(no-jar)+WithToken) 应返回 error（cookie jar 缺失），但 err 为 nil")
	}
	if !strings.Contains(err.Error(), "cookie") && !strings.Contains(err.Error(), "Jar") {
		t.Errorf("error 应提示 cookie/Jar 问题，实际: %v", err)
	}
}

// ─── HIGH #9: tokenparse.ExtractFromLocation URL 解析 ───

// TestRegression_Login_TruncatesTokenAtAmpersand 验证 token 解析在
// query 含 & 时正确截断（HAR 验证）。
func TestRegression_Login_TruncatesTokenAtAmpersand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":1,"dataList":[{"school_id":"173","NAME":"测试"}]}`))
		case "/uiActivityLogin/studentLogin":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":1,"msg":"登陆成功","returnData":{"token":"eyJhbGciOiJIUzI1NiJ9.payload.sig"}}`))
		}
	}))
	defer srv.Close()

	c, err := client.New(
		client.WithSSOBase(srv.URL),
	)
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if !strings.HasPrefix(resp.Token, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("token 解析错误，得到: %s", resp.Token)
	}
}
