package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ─── login 限流/服务端故障专属文案可达性 ───

// mockLoginSSO 构造覆盖 GetSchoolID → studentLogin 全链的 mock SSO：
// getSchoolIdByStudentNumber 恒 200 成功，studentLogin 按参数返回指定状态码。
// 只有走完 GetSchoolID 后 studentLogin 才可能拿到 429/5xx，错误链才会只含
// ErrRateLimited/ErrServiceUnavailable（不含 ErrLoginRejected）——真实复现
// 死代码根因。
func mockLoginSSO(t *testing.T, loginCode int, loginBody string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[{"school_id":"173","NAME":"示例中学"}]}`))
		case "/uiActivityLogin/studentLogin":
			w.WriteHeader(loginCode)
			_, _ = w.Write([]byte(loginBody))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newLoginTestCmd(srvURL string) *cobra.Command {
	cmd := &cobra.Command{Use: "login"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("username", "", "")
	_ = cmd.Flags().Set("username", "student001")
	cmd.Flags().String("password", "", "")
	_ = cmd.Flags().Set("password", "pass")
	cmd.Flags().String("sso-base", "", "")
	_ = cmd.Flags().Set("sso-base", srvURL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestLoginCmd_RateLimited_ShowsDedicatedMessage 锁定
// 登录被限流（SDK 返回 ErrRateLimited，不含 ErrLoginRejected）时，login 命令
// 必须渲染「请求被限流」专属中文文案 + envelope 429。修复前外层 errors.Is
// ErrLoginRejected 永假（内层限流分支是死代码），专属文案永不渲染，错误走
// default 分支以 printError 原样展示到 stderr。
func TestLoginCmd_RateLimited_ShowsDedicatedMessage(t *testing.T) {
	srv := mockLoginSSO(t, http.StatusTooManyRequests, `{"code":-1,"msg":"rate limited"}`)

	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, _, restore := captureStdio(t)
	loginCmd.Run(newLoginTestCmd(srv.URL), nil)
	restore()
	stdout := stdoutBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("限流应落业务错误档 exit 1，实际 %d; stdout=%q", got, stdout)
	}
	if !strings.Contains(stdout, "请求被限流") {
		t.Errorf("限流应显示专属中文文案「请求被限流」，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"code": 429`) {
		t.Errorf("限流 envelope 应为 429，实际: %q", stdout)
	}
}

// TestLoginCmd_ServiceUnavailable_ShowsDedicatedMessage 是限流分支的另一半
// 5xx 服务端故障（ErrServiceUnavailable）时命中「SSO 服务端暂时不可用」专属分支，
// 而不是 default 兜底。与限流分支同属修复前死代码族。
func TestLoginCmd_ServiceUnavailable_ShowsDedicatedMessage(t *testing.T) {
	srv := mockLoginSSO(t, http.StatusBadGateway, `502 Bad Gateway`)

	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, _, restore := captureStdio(t)
	loginCmd.Run(newLoginTestCmd(srv.URL), nil)
	restore()
	stdout := stdoutBuf.String()

	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("服务端故障应落服务端错误档 exit 2，实际 %d; stdout=%q", got, stdout)
	}
	if !strings.Contains(stdout, "暂时不可用") {
		t.Errorf("服务端故障应显示专属中文文案「暂时不可用」，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"code": 502`) {
		t.Errorf("服务端故障 envelope 应为 502，实际: %q", stdout)
	}
}
