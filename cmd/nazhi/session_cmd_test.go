package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// makeSessionActivateTestCmd 创建 session activate 命令的测试用 cobra.Command + mock server。
//
// getMyInfoBody: /api/studentInfo/getMyInfo 的响应体 JSON
//   - 空字符串: 默认 returnData/dataMap 都为 nil（触发空响应路径）
//   - 正常 JSON: returnData 含 name/studentNumber 等字段
func makeSessionActivateTestCmd(t *testing.T, token string, getMyInfoBody string) (*cobra.Command, *client.Client) {
	t.Helper()
	if getMyInfoBody == "" {
		getMyInfoBody = `{"code":1,"msg":"成功"}`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(getMyInfoBody))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		}
	}))
	t.Cleanup(srv.Close)

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithSSOBase(srv.URL),
		client.WithToken(token),
	)
	trackClient(c)
	t.Cleanup(func() { _ = c.Close() })

	cmd := &cobra.Command{Use: "session-activate"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", token, "")
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")
	return cmd, c
}

// TestSessionActivate_EmptyUserInfo_StatusEnvelope 回归测试 F10：
// session activate 命令在 ActivateSession 返回 (nil, ErrEmptyUserInfo) 时
// 必须输出 status envelope，**不**输出裸 null。
//
// 失败场景（修复前）：cmd/nazhi/session.go:38 printJSON(info) → 输出 `null\n`
// 与 whoami 的 {status: empty, reason: ...} 不一致。
//
// 修复后：cmd 层用 errors.Is(err, ErrEmptyUserInfo) 分支输出 status envelope。
func TestSessionActivate_EmptyUserInfo_StatusEnvelope(t *testing.T) {
	cmd, _ := makeSessionActivateTestCmd(t, "test-token", "")

	quiet = false
	pendingExitCode.Store(0)

	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// 关键：sessionActivateCmd.Run 是闭包，调 sessionActivateCmd.Run(cmd, nil)
	sessionActivateCmd.Run(cmd, nil)

	_ = wOut.Close()
	_ = wErr.Close()
	outDone := make(chan struct{})
	errDone := make(chan struct{})
	go func() { _, _ = io.Copy(stdoutBuf, rOut); close(outDone) }()
	go func() { _, _ = io.Copy(stderrBuf, rErr); close(errDone) }()
	<-outDone
	<-errDone
	os.Stdout, os.Stderr = origStdout, origStderr

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码保持 0（不是 error 路径）
	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("空响应 session activate 不应触发 pendingExitCode=1，实际 %d", got)
	}

	// stderr 不应有 error 标记
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON（F10 修复后输出 status envelope），实际: %q", stderr)
	}

	// stdout 应输出 status envelope 而非裸 null
	if strings.TrimSpace(stdout) == "null" {
		t.Errorf("stdout 不应是裸 null（F10 修复后输出 status 对象），实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"status": "empty"`) {
		t.Errorf("stdout 应包含 status: empty，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"reason": "get_my_info_empty"`) {
		t.Errorf("stdout 应包含 reason: get_my_info_empty，实际: %q", stdout)
	}
}

// TestSessionActivate_ValidUserInfo_OutputsUserInfo 验证正常响应时
// session activate 直接输出 UserInfo（向后兼容 happy path）。
func TestSessionActivate_ValidUserInfo_OutputsUserInfo(t *testing.T) {
	validBody := `{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`
	cmd, _ := makeSessionActivateTestCmd(t, "test-token", validBody)

	quiet = false
	pendingExitCode.Store(0)

	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	sessionActivateCmd.Run(cmd, nil)

	_ = wOut.Close()
	_ = wErr.Close()
	outDone := make(chan struct{})
	errDone := make(chan struct{})
	go func() { _, _ = io.Copy(stdoutBuf, rOut); close(outDone) }()
	go func() { _, _ = io.Copy(stderrBuf, rErr); close(errDone) }()
	<-outDone
	<-errDone
	os.Stdout, os.Stderr = origStdout, origStderr

	stdout := stdoutBuf.String()

	if !strings.Contains(stdout, `"name": "张三"`) {
		t.Errorf("正常响应 stdout 应包含 name: 张三，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"studentNumber": "TEST2025001"`) {
		t.Errorf("正常响应 stdout 应包含 studentNumber，实际: %q", stdout)
	}
}

// 防止 errors 包未使用（编译静态检查）
var _ = errors.Is
