// Package client 内部白盒测试。
//
// F1 (review-tdd 二轮): pkg/client/auth.go 200 路径缺 F2 expiresAt 兜底 warn — 回归测试。
//
// 历史 bug（review-tdd 一轮 F2 仅修了 302 路径，200 路径漏修）：
//   - 302 路径 (auth.go:189-191) 有 `if time.Until(expiresAt) > 23*time.Hour {
//     c.logger.Warn(...) }`：server 未给 expires_in/exp 时兜底 now+24h 必 warn。
//   - 200 路径 (auth.go:156) 调用 extractTokenFromReturnData，该函数 (auth.go:354-369)
//     **总是**返回 time.Now().Add(24*time.Hour)（不解析 returnData 里的 exp/expires_in），
//     但调用方 Login 函数完全没有对应的 warn。
//
// 实际影响：200 路径的 token 24h 后神秘失效，但 stderr 完全没有日志。
// 用户排查 "token 为什么突然 401" 时无从下手。
//
// 修复后：200 路径补对称的 warn 日志，与 302 路径语义一致。
//
// 验证策略：构造一个 server 返回 200 + UnifiedResponse{code=1, returnData={token:"jwt"}}
// （**不**包含 exp/expires_in 字段），断言日志必须以 WARN 级别输出兜底告警。
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

// TestLogin_200Path_ExpiresAtFallback_LogsAtWarn 验证 200 路径触发 expiresAt
// 兜底（now+24h）时，告警必须以 WARN 级别输出，与 302 路径语义对称。
//
// 场景：server 返回 200 + UnifiedResponse，returnData 含 token 但**无**exp/expires_in
// 字段（HAR 验证的登录响应现状，server 不带过期信息）。
// extractTokenFromReturnData 返回 now+24h → Login 应 Warn 提示。
//
// 修复前：完全静默（200 路径无任何 expiresAt warn 代码）。
//
// 修复后：c.logger.Warn → 默认 LevelWarn 下用户立即知道 server 行为异常。
func TestLogin_200Path_ExpiresAtFallback_LogsAtWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			// InitSession: 任意 200 即可
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			// 验证码图片: 任意非空字节
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			// 预校验验证码: 业务成功
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 登录: 200 + UnifiedResponse，returnData 含 token 但**无**exp/expires_in
			// （HAR 验证的现状：server 不带过期信息，200 路径永远走 now+24h 兜底）。
			// 注意：returnData 是嵌套 JSON 对象（json.RawMessage），不是字符串。
			w.Header().Set("Content-Type", "application/json")
			// 关键：returnData 只有 token 字段，没有 exp/expires_in
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"token":"jwt-no-expires"}}`))
		}
	}))
	defer srv.Close()

	// 自定义 logger: bytes.Buffer 收集所有日志，LevelDebug 让 Debug 也可见
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

	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173", // 跳过 GetSchoolID
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if resp.Token != "jwt-no-expires" {
		t.Errorf("token 应为 'jwt-no-expires'，实际: %s", resp.Token)
	}

	// 关键断言：日志必须以 WARN 级别输出兜底告警
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "level=WARN") {
		t.Errorf("期望 WARN 级别日志（让默认 LevelWarn 下用户也能看见），实际日志:\n%s", logOutput)
	}
	// 断言内容含兜底语义（与 302 路径一致）
	if !strings.Contains(logOutput, "24h") && !strings.Contains(logOutput, "兜底") && !strings.Contains(logOutput, "fallback") {
		t.Errorf("兜底告警应说明 '24h 兜底' 语义，实际日志:\n%s", logOutput)
	}
	// 断言是 200 路径（带 "200" 标识符以便区分 302 路径告警）
	if !strings.Contains(logOutput, "200") {
		t.Errorf("兜底告警应包含 '200' 路径标识符，实际日志:\n%s", logOutput)
	}
}
