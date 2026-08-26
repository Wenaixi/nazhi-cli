package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestLogin_ValidateOversizedBody_Rejects 锁定 http-infra 域 19 轮审计 P2-1：
// Login validate 端点响应体 io.ReadAll 无上限（auth.go:168），与全仓限读纪律
// （httpDo 1MB / file.go 64KB+1MB / doGetMenu 100B）不符。异常/被劫持 SSO 塞超大
// body 时无条件全读入内存。修复后：超限归 ErrLoginRejected（与 200 分支既有哨兵一致）。
// 夹具关键：响应必须是超大且**合法 JSON**（token 有效）——若用非法 JSON 会在
// 裸 ReadAll 后 DecodeResponse 失败同样归 ErrLoginRejected，测试退化为恒真。
func TestLogin_ValidateOversizedBody_Rejects(t *testing.T) {
	// 5MB 合法 JSON：{"code":1,"msg":"ok","returnData":{"token":"<jwt with padding>"}}
	bigToken := strings.Repeat("x", 5<<20) // 5MB token 填充
	loginBody := fmt.Sprintf("{\"code\":1,\"msg\":\"成功\",\"returnData\":{\"token\":\"%s\"}}", bigToken)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\"}"))
		case "/teacher/auth/studentLogin/validate":
			// 异常/被劫持服务端返回超大但合法的 200 JSON
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(loginBody))
		}
	}))
	defer srv.Close()

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     nil,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("超大响应体应报错，实际 nil")
	}
	if !errors.Is(err, ErrLoginRejected) {
		t.Errorf("超大响应体应归 ErrLoginRejected，实际 %v", err)
	}
}