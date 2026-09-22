package client

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// newClientForLoginTest 为免验证码登录测试构造 Client（无 OCR 字段）。
func newClientForLoginTest(ssoURL string) *Client {
	return &Client{
		ssoBaseURL: ssoURL,
		baseURL:    ssoURL,
		uploadURL:  ssoURL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
	}
}

// TestLogin_ActivityEndpoint_UsesMd5Password 锁定新接线：Login 走五育免验证码
// 端点，请求体带 md5(password)，不再访问任何 kaptcha/validateCaptcha/validate。
func TestLogin_ActivityEndpoint_UsesMd5Password(t *testing.T) {
	var (
		reqBody  map[string]string
		gotPath  string
		wrongPwd bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"173","NAME":"示例中学"}]}`))
		case "/uiActivityLogin/studentLogin":
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			if reqBody["password"] != "2f164535ed4936b73adcfa8df1aaffc6" {
				wrongPwd = true
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"登陆成功","returnData":{"token":"md5-jwt-token"}}`))
		default:
			t.Errorf("未预期路径 %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newClientForLoginTest(srv.URL)
	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "TEST2025001",
		Password: "714348",
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if resp.Token != "md5-jwt-token" {
		t.Errorf("token 错: %q", resp.Token)
	}
	if gotPath != "/uiActivityLogin/studentLogin" {
		t.Errorf("应请求五育端点，实际 %q", gotPath)
	}
	if reqBody["username"] != "TEST2025001" || reqBody["schoolId"] != "173" {
		t.Errorf("body 错: %v", reqBody)
	}
	if wrongPwd {
		t.Errorf("password 应为 md5(714348)，实际 %q", reqBody["password"])
	}
}
