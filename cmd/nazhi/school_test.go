package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeSchoolTestCmd 创建 school 命令的测试用 cobra.Command + mock SSO server。
// username 为空时不设 --username flag，用于测试缺省 / env fallback 场景。
func makeSchoolTestCmd(t *testing.T, username string) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// GetSchoolID 的响应：code=1 + dataList 含学校信息
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[{"school_id":100,"NAME":"测试学校"}]}`))
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "school"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("username", "", "")
	if username != "" {
		_ = cmd.Flags().Set("username", username)
	}
	cmd.Flags().String("sso-base", "", "")
	_ = cmd.Flags().Set("sso-base", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestSchoolCmd_WithUsername 验证 --username 正确传递并输出 school_id + school_name JSON。
func TestSchoolCmd_WithUsername(t *testing.T) {
	cmd := makeSchoolTestCmd(t, "TEST2025001")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	schoolCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("正常路径不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"school_id": "100"`) {
		t.Errorf("stdout 应包含 school_id: \"100\"，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"school_name": "测试学校"`) {
		t.Errorf("stdout 应包含 school_name，实际: %q", stdout)
	}
}

// TestSchoolCmd_MissingUsername_PrintsError 验证 --username 缺省时输出 error JSON 到 stderr。
func TestSchoolCmd_MissingUsername_PrintsError(t *testing.T) {
	_ = os.Unsetenv("NAZHI_USERNAME")

	cmd := makeSchoolTestCmd(t, "") // 不设 username

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	schoolCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("缺 username 应触发 pendingExitCode=1，实际 %d", got)
	}
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stderr, "username") {
		t.Errorf("stderr 应包含 username 提示，实际: %q", stderr)
	}
	_ = stdout
}

// TestSchoolCmd_EnvFallback 验证 NAZHI_USERNAME 环境变量作为 fallback 生效。
func TestSchoolCmd_EnvFallback(t *testing.T) {
	t.Setenv("NAZHI_USERNAME", "ENV2025")

	cmd := makeSchoolTestCmd(t, "") // 不设 flag，走 env fallback

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	schoolCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("env fallback 不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("env fallback 路径的 stderr 不应含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"school_id": "100"`) {
		t.Errorf("env fallback 应输出 school_id: \"100\"，实际: %q", stdout)
	}
}
