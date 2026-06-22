// Package client_test 包含针对已审查 bug 的回归测试。
//
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

	c := client.New(
		client.WithBaseURL(srv.URL),
		client.WithToken("fake"),
	)
	_, err := c.QuerySelfGradEvaluation(context.Background(), "fake")
	if err == nil {
		t.Fatal("期望返回错误（响应体无效 JSON），但 err 为 nil")
	}
	if !strings.Contains(err.Error(), "QuerySelfGradEvaluation") {
		t.Errorf("错误信息应包含函数名，便于定位: %v", err)
	}
}

// ─── HIGH #6: UploadFile 独立 client 应禁用自动重定向 ───

// TestRegression_UploadFile_NoRedirectFollow 验证上传文件时遇到 302
// 不会自动跟随（防止请求发到错误主机）。
func TestRegression_UploadFile_NoRedirectFollow(t *testing.T) {
	attackerHit := atomic.Bool{}
	uploadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 故意返回 302
		w.Header().Set("Location", "http://attacker.invalid/steal")
		w.WriteHeader(http.StatusFound)
		attackerHit.Store(true)
	}))
	defer uploadSrv.Close()

	c := client.New(
		client.WithUploadURL(uploadSrv.URL),
		client.WithSSOBase("http://sso.example"),
	)
	tmpFile := t.TempDir() + "/test.png"
	if err := writeSimplePNG(tmpFile); err != nil {
		t.Fatal(err)
	}

	_, err := c.UploadFile(context.Background(), tmpFile)
	// 应该返回错误（302 非 200），而不是成功
	if err == nil {
		t.Fatal("UploadFile 在 302 时应返回错误，但 err 为 nil")
	}
	if !strings.Contains(err.Error(), "302") && !strings.Contains(err.Error(), "status=") {
		t.Errorf("错误信息应指出 302 状态: %v", err)
	}
	// 读取 atomic.Bool 避免 vet 报的 noCopy 警告
	_ = attackerHit.Load()
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
func TestRegression_WithHTTPClient_NoJar_DoesNotPanic(t *testing.T) {
	customHTTP := &http.Client{} // 无 Jar

	c := client.New(
		client.WithHTTPClient(customHTTP),
		client.WithToken("test-token"),
	)
	if c == nil {
		t.Fatal("New returned nil")
	}
	// 验证：调用 New 不会 panic，syncCookieToken 能优雅处理 nil jar
}

// ─── HIGH #9: extractTokenFromLocation URL 解析 ───

// TestRegression_Login_TruncatesTokenAtAmpersand 验证 token 解析在
// query 含 & 时正确截断（HAR 验证）。
func TestRegression_Login_TruncatesTokenAtAmpersand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html>login</html>"))
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":1,"dataList":[{"school_id":"173","NAME":"测试"}]}`))
		case "/kaptcha/kaptcha.jpg":
			w.Write([]byte{0xFF, 0xD8, 0xFF})
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"code":1,"msg":"ok"}`))
		case "/teacher/auth/studentLogin/validate":
			// 302 with token + extra query params
			w.Header().Set("Location", "/homepage?token=eyJhbGciOiJIUzI1NiJ9.payload.sig&foo=bar&baz=qux")
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer srv.Close()

	c := client.New(
		client.WithSSOBase(srv.URL),
		client.WithCustomOCR(&mockOCR{text: "AB12"}),
	)
	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if !strings.HasPrefix(resp.Token, "eyJhbGciOiJIUzI1NiJ9") {
		t.Errorf("token 解析错误，得到: %s", resp.Token)
	}
	if strings.Contains(resp.Token, "&") || strings.Contains(resp.Token, "foo") || strings.Contains(resp.Token, "baz") {
		t.Errorf("token 应在第一个 & 处截断，得到: %s", resp.Token)
	}
}
