// Package client 内部白盒测试。
//
// F6: pkg/client/auth.go:142 200 路径吞掉 unmarshal 错误 — 回归测试。
//
// 历史 bug：line 143 的 if err := json.Unmarshal(...); err == nil 守卫
// 吞掉 unmarshal 错误，line 149 的 if err == nil 同样吞掉
// extractTokenFromReturnData 错误。错误信息只说"未找到 token"，
// 丢失关键诊断上下文。line 191-194 的非预期状态码路径已正确处理
// 但 200 路径漏了。
//
// 修复后：在 200 路径加 logDebug 处理 unmarshal/extractToken 错误，
// 风格与 line 191-194 一致（保留原始 body 摘要便于排查）。
//
// 验证策略：构造一个 server 返回 200 + 非 UnifiedResponse JSON 的 body
// （例如空对象 {} 或非 JSON 字符串），断言 logDebug 输出包含原始 body
// 摘要和"解析失败"语义。
package client

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestLogin_200Path_LogsUnmarshalFailure 验证 HTTP 200 + body 解析失败时，
// 必须 logDebug 输出原始 body 摘要（便于排查非 UnifiedResponse 错误响应）。
//
// 场景 1：body 是空对象 {} → json.Unmarshal 成功但 loginResp.ReturnData 为 nil
//
//	→ extractTokenFromReturnData 返回 "returnData 为空" 错误
//	→ 当前实现：吞掉错误，错误信息只说"未找到 token"
//	→ 修复后：logDebug 输出 body + 错误
func TestLogin_200Path_LogsUnmarshalFailure(t *testing.T) {
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
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 关键：返回 200 + 空对象 {} → json.Unmarshal 成功但无 token 字段
			w.Header().Set("Content-Type", "application/json")
			// 返回 returnData=null 让 extractTokenFromReturnData 失败
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null}`))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "未找到 token") {
		t.Errorf("期望 '未找到 token' 错误，实际: %v", err)
	}

	// 关键断言：logDebug 必须输出原始 body 摘要 + 错误原因
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Login 200") {
		t.Errorf("logDebug 应输出 'Login 200' 标识符便于排查，实际日志:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "body=") {
		t.Errorf("logDebug 应包含 body= 字段便于查看原始 body，实际日志:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "returnData") {
		t.Errorf("logDebug 应包含 body 内容 'returnData'（mock body 的标识），实际日志:\n%s", logOutput)
	}
}

// TestLogin_200Path_LogsNonJSONBody 验证 HTTP 200 + body 不是 JSON 时
// （例如 HTML 错误页），必须 logDebug 输出 body 摘要。
//
// 场景：server 返回 200 + HTML 错误页（中间件拦截）。
// json.Unmarshal 失败 → 当前实现：吞掉错误，错误信息只说"未找到 token"
// 修复后：logDebug 输出 "解析失败" + body 摘要。
func TestLogin_200Path_LogsNonJSONBody(t *testing.T) {
	const htmlBody = "<html><body>500 Internal Server Error</body></html>"

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
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(htmlBody))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误，实际 nil")
	}

	// 关键断言：logDebug 必须输出 body 摘要
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Login 200") {
		t.Errorf("logDebug 应输出 'Login 200' 标识符便于排查，实际日志:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "body=") {
		t.Errorf("logDebug 应包含 body= 字段便于查看原始 body，实际日志:\n%s", logOutput)
	}
	// 验证 body 摘要有意义（应包含 body 的一部分内容）
	bodyContainsHTML := strings.Contains(logOutput, "500") ||
		strings.Contains(logOutput, "html") ||
		strings.Contains(logOutput, "Internal")
	if !bodyContainsHTML {
		t.Errorf("logDebug 的 body 摘要应包含原 body 内容（HTML 500 错误页），实际日志:\n%s", logOutput)
	}
}
