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

// makeSelfEvalSubmitTestCmd 创建 self-eval submit 命令的测试用 cobra.Command + mock server。
// comment 是 --comment flag 的值（空字符串时不设，用于测试 stdin 读入场景）。
func makeSelfEvalSubmitTestCmd(t *testing.T, comment string) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"测试学校","className":"高一八班","seat":45}}`))
		case "/api/studentMoralEduNew/addSelfEvaluation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"msg":"未知路径"}`))
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "self-eval-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("comment", "", "")
	if comment != "" {
		_ = cmd.Flags().Set("comment", comment)
	}
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestSelfEvalSubmitCmd_WithComment 验证 --comment flag 正常提交并输出成功 JSON。
func TestSelfEvalSubmitCmd_WithComment(t *testing.T) {
	cmd := makeSelfEvalSubmitTestCmd(t, "很好的学期")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("正常路径不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "ok"`) {
		t.Errorf("stdout 应包含 status: ok，实际: %q", stdout)
	}
}

// TestSelfEvalSubmitCmd_StdinPipe 验证 --comment "" 时从 stdin 读取评价内容。
// 测试环境下 stdin 是管道，ReadString(0) 读到 EOF 返回写入的内容。
func TestSelfEvalSubmitCmd_StdinPipe(t *testing.T) {
	cmd := makeSelfEvalSubmitTestCmd(t, "") // comment 不设，触发 stdin 读入

	// 创建 pipe 并写入评价内容
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	_, _ = w.WriteString("从 stdin 读取的评价")
	_ = w.Close()
	defer func() { os.Stdin = origStdin }()

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("stdin 读入不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "ok"`) {
		t.Errorf("stdout 应包含 status: ok，实际: %q", stdout)
	}
}

// TestSelfEvalSubmitCmd_EmptyStdin_PrintsError 验证 stdin 为空（无输入）时输出错误。
func TestSelfEvalSubmitCmd_EmptyStdin_PrintsError(t *testing.T) {
	cmd := makeSelfEvalSubmitTestCmd(t, "") // comment 不设

	// 创建空 stdin pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	_ = w.Close() // 立即关闭，模拟空输入
	defer func() { os.Stdin = origStdin }()

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 空 stdin → envelope.Error(400, ...) → pendingExitCode=3（参数错误）
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("空 stdin 应触发 pendingExitCode=3（envelope.Error(400)），实际 %d", got)
	}
	if !strings.Contains(stdout, `"status": "error"`) {
		t.Errorf("stdout 应包含 envelope.Error，实际: %q", stdout)
	}
	if !strings.Contains(stdout, "评价内容不能为空") {
		t.Errorf("stdout 应包含空评价提示，实际: %q", stdout)
	}
	_ = stderr
}

// TestSelfEvalSubmitCmd_StdinReadError_Propagates 验证 stdin 读取发生 I/O 错误时
// （如 fd 关闭、管道断开），真实错误被传播到 stderr，而非被掩盖。
// 回归测试：ReadString 的 error 曾被 _ 丢弃。
func TestSelfEvalSubmitCmd_StdinReadError_Propagates(t *testing.T) {
	cmd := makeSelfEvalSubmitTestCmd(t, "") // comment 不设，触发 stdin 读入

	// 创建一个已关闭的文件替换 stdin，ReadString 将返回非 io.EOF 的 I/O 错误
	f, err := os.CreateTemp("", "stdin-closed-*")
	if err != nil {
		t.Fatalf("CreateTemp 失败: %v", err)
	}
	_ = f.Close()
	_ = os.Remove(f.Name())

	origStdin := os.Stdin
	os.Stdin = f // 用已关闭的文件替换 stdin
	defer func() { os.Stdin = origStdin }()

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalSubmitCmd.Run(cmd, nil)
	restore()
	stderr := stderrBuf.String()

	// stdin I/O 错误走 printError → envelope.ExitCode=2
	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("stdin 读错误应触发 pendingExitCode=2（printError 走 500），实际 %d", got)
	}
	if !strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 应包含 error envelope，实际: %q", stderr)
	}
	// 核心断言：真实 I/O 错误不应被掩盖为"评价内容不能为空"
	if strings.Contains(stderr, "评价内容不能为空") {
		t.Errorf("真实 I/O 错误被掩盖为评价内容不能为空: %q", stderr)
	}
	if !strings.Contains(stderr, "读取 stdin 评价内容失败") {
		t.Errorf("stderr 应包含 stdin 读取失败的真实错误，实际: %q", stderr)
	}
	_ = stdoutBuf
}

// TestSelfEvalSubmitCmd_ServerError 验证服务端业务错误传播。
func TestSelfEvalSubmitCmd_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"测试学校","className":"高一八班","seat":45}}`))
		case "/api/studentMoralEduNew/addSelfEvaluation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":500,"msg":"提交失败"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cmd := &cobra.Command{Use: "self-eval-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("comment", "测试评价", "")
	_ = cmd.Flags().Set("comment", "测试评价")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 业务错误应触发 pendingExitCode=2（envelope.Error 5xx → exit code 2）
	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("业务错误应触发 pendingExitCode=2，实际 %d", got)
	}
	if !strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 应包含 error envelope，实际: %q", stderr)
	}
	_ = stdout
}
