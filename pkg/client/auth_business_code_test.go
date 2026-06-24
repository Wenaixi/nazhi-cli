package client

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestLogin_200WithBusinessErrorCode 验证 HTTP 200 + 业务错误码时，
// 应返回包含业务 msg 的错误（如"密码错误"），而不是低语义的"未找到 token"。
//
// Bug 场景：server 返回 200 + {"code":2,"msg":"密码错误"} → 之前会丢失业务信息。
func TestLogin_200WithBusinessErrorCode(t *testing.T) {
	// 业务错误只在登录 POST 时返回，其他路径返回成功
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			// InitSession: 任意 200 即可
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html>ok</html>`))
		case "/kaptcha/kaptcha.jpg":
			// 验证码图片: 任意非空字节
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			// 预校验验证码: 业务成功
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 登录: 200 + 业务错误码（关键测试场景）
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":2,"msg":"密码错误"}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		}
	}))
	defer srv.Close()

	// 内部包：直接构造 Client + 注入 mock OCR + 提供 SchoolID 跳过 GetSchoolID
	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "user",
		Password: "pass",
		SchoolID: "173", // 跳过 GetSchoolID
	})

	if err == nil {
		t.Fatal("期望返回业务错误，实际 nil")
	}
	// 必须含业务 msg，不能只是"未找到 token"
	if !strings.Contains(err.Error(), "密码错误") {
		t.Errorf("错误信息应包含业务 msg '密码错误'，实际: %v", err)
	}
	if strings.Contains(err.Error(), "未找到 token") {
		t.Errorf("错误信息不应是低语义的'未找到 token'，实际: %v", err)
	}
}
