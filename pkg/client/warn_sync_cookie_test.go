// Package client 内部白盒测试。
// F2 ( 二轮): pkg/client/auth.go syncCookieToken 200/302 路径 copy-paste 去重 — 回归测试。
// 历史 bug：200 路径 (auth.go:166-168) 和 302 路径 (auth.go:194-196) 都有
//
//	if err := c.syncCookieToken(token); err != nil {
//	 c.logger.Warn("Login <label> 后同步 token 到 cookie 失败", "err", err.Error())
//	}
//
// 两段相同代码 copy-paste，仅 label 不同。修改时容易漏改一边，行为漂移风险高。
// 修复后：提取 c.warnSyncCookieToken(token, label) helper，200/302 路径
// 共用同一实现。
// 验证策略：
// 1. 直接测试 helper：传入错误路径场景（non-jar http.Client）→ 断言输出
// WARN 日志 + 含 label 标识符
// 2. 反向断言：helper 的日志不应包含 token 字面值（避免凭据泄露）
package client

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestWarnSyncCookieToken_BadJar_LogsWarn 验证 helper 在 cookie 同步失败时
// 输出 WARN 日志且包含调用方提供的 label 标识符。
// 场景：自定义 http.Client（Jar 为 nil，非 *cookiejar.Jar）→ syncCookieToken
// 返回 error → helper 应输出 WARN 日志。
// 修复前：200/302 两段 copy-paste，仅 label 字符串不同。
// 修复后：统一走 helper，label 由调用方传入。
func TestWarnSyncCookieToken_BadJar_LogsWarn(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       &http.Client{Timeout: 5 * time.Second}, // Jar = nil → syncCookieToken 返回 error
		logger:     logger,
	}

	// 调用 helper，label = "TEST_LABEL"
	c.warnSyncCookieToken("dummy-token", "TEST_LABEL")

	logOutput := logBuf.String()

	// 关键断言 1：必须以 WARN 级别输出（默认 LevelWarn 下用户可见）
	if !strings.Contains(logOutput, "level=WARN") {
		t.Errorf("期望 WARN 级别日志，实际日志:\n%s", logOutput)
	}
	// 关键断言 2：必须包含调用方传入的 label 标识符（200 / 302 fallback 等）
	if !strings.Contains(logOutput, "TEST_LABEL") {
		t.Errorf("日志应包含 label 标识符 'TEST_LABEL'，实际日志:\n%s", logOutput)
	}
	// 关键断言 3：日志消息前缀应是 "Login <label> 后同步 token 到 cookie 失败"
	if !strings.Contains(logOutput, "Login TEST_LABEL") {
		t.Errorf("日志消息应以 'Login TEST_LABEL' 开头，实际日志:\n%s", logOutput)
	}
	// 关键断言 4：err 字段应包含 syncCookieToken 的具体错误
	if !strings.Contains(logOutput, "cookie") && !strings.Contains(logOutput, "Jar") {
		t.Errorf("err 字段应包含 cookie/Jar 相关错误信息，实际日志:\n%s", logOutput)
	}
}

// TestWarnSyncCookieToken_BadJar_DoesNotLeakToken 验证 helper 输出错误日志时
// **不会** 把 token 写入日志（避免敏感凭据泄露到 stderr）。
// 安全约束：token 是 X-Auth-Token，业务调用方常把日志收集到 ELK / 第三方，
// 一旦泄露等同于泄露登录态。F2 helper 必须保证失败日志不含 token 字面值。
func TestWarnSyncCookieToken_BadJar_DoesNotLeakToken(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       &http.Client{Timeout: 5 * time.Second}, // Jar = nil
		logger:     logger,
	}

	// 使用一个足够独特的 token 字符串，便于断言是否泄露
	const secretToken = "SECRET_TOKEN_XYZ_DO_NOT_LEAK_42"
	c.warnSyncCookieToken(secretToken, "leak-check")

	logOutput := logBuf.String()

	// 反向断言：日志不应包含 token 字面值
	if strings.Contains(logOutput, secretToken) {
		t.Errorf("FAIL: 日志泄露了 token 字符串！实际日志:\n%s", logOutput)
	}
}
