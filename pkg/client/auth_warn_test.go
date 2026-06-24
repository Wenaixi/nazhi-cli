// Package client 内部白盒测试。
//
// F2: pkg/client/auth.go:174 expiresAt 兜底从 Warn 降级为 logDebug — 回归测试。
//
// 历史 bug：注释自述"warn 出来"但实现用 c.logDebug() 输出兜底告警。
// 默认 logger 级别是 slog.LevelWarn，Debug 被过滤。普通 CLI 调用
// 完全静默——用户拿到 24h 后神秘失效的 token 而无任何告警。
//
// 修复后：c.logDebug 改回 c.logger.Warn（与 line 191-194 模式一致）。
//
// 验证策略：用 bytes.Buffer + slog.LevelDebug 让所有日志都可见，
// 构造一个 302 + Location 无 expires 的 server，断言"24h 兜底"
// 告警以 WARN 级别输出。
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

// TestLogin_302Fallback_ExpiresAtFallback_LogsAtWarn 验证 302 fallback
// 路径触发 expiresAt 兜底（now+24h）时，告警必须以 WARN 级别输出，
// 而不是 Debug（默认 LevelWarn 下被过滤，用户完全看不见）。
//
// 场景：server 返回 302 + Location 含 token 但无 expires_in/exp。
// Login 解析 Location → expiresAt = now+24h（兜底）→ 应 Warn 提示。
//
// 修复前：c.logDebug("...") → 默认 LevelWarn 下被过滤 → 静默。
// 修复后：c.logger.Warn("...") → 永远可见 → 用户立即知道 server 行为异常。
func TestLogin_302Fallback_ExpiresAtFallback_LogsAtWarn(t *testing.T) {
	// 启动 server：让 validate 路径返回 302 + Location 无 expires
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
			// 登录: 302 + Location 含 token 但无 expires 参数
			w.Header().Set("Location", "/homepage?token=jwt-no-expires")
			w.WriteHeader(http.StatusFound)
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
	// 反向断言：不应只输出 Debug
	if !strings.Contains(logOutput, "level=WARN") && strings.Contains(logOutput, "level=DEBUG") {
		t.Errorf("兜底告警不应只用 Debug（默认 LevelWarn 下会被过滤），实际日志:\n%s", logOutput)
	}
	// 断言内容含兜底语义
	if !strings.Contains(logOutput, "24h") && !strings.Contains(logOutput, "兜底") && !strings.Contains(logOutput, "fallback") {
		t.Errorf("兜底告警应说明 '24h 兜底' 语义，实际日志:\n%s", logOutput)
	}
}