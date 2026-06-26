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
func makeSelfEvalSubmitTestCmd(t *testing.T, comment string) (*cobra.Command, *httptest.Server) {
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
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")
	return cmd, srv
}

// TestSelfEvalSubmitCmd_WithComment 验证 --comment flag 正常提交并输出成功 JSON。
func TestSelfEvalSubmitCmd_WithComment(t *testing.T) {
	cmd, _ := makeSelfEvalSubmitTestCmd(t, "很好的学期")

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
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "ok"`) {
		t.Errorf("stdout 应包含 status: ok，实际: %q", stdout)
	}
}

// TestSelfEvalSubmitCmd_StdinPipe 验证 --comment "" 时从 stdin 读取评价内容。
// 测试环境下 stdin 是管道，ReadString(0) 读到 EOF 返回写入的内容。
func TestSelfEvalSubmitCmd_StdinPipe(t *testing.T) {
	cmd, _ := makeSelfEvalSubmitTestCmd(t, "") // comment 不设，触发 stdin 读入

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
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "ok"`) {
		t.Errorf("stdout 应包含 status: ok，实际: %q", stdout)
	}
}

// TestSelfEvalSubmitCmd_EmptyStdin_PrintsError 验证 stdin 为空（无输入）时输出错误。
func TestSelfEvalSubmitCmd_EmptyStdin_PrintsError(t *testing.T) {
	cmd, _ := makeSelfEvalSubmitTestCmd(t, "") // comment 不设

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

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("空 stdin 应触发 pendingExitCode=1，实际 %d", got)
	}
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stderr, "评价内容不能为空") {
		t.Errorf("stderr 应包含空评价提示，实际: %q", stderr)
	}
	_ = stdout
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
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("业务错误应触发 pendingExitCode=1，实际 %d", got)
	}
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 应包含 error JSON，实际: %q", stderr)
	}
	_ = stdout
}
