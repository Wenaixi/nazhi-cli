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

// makeSelfEvalGradSubmitStdinTestCmd 构造 grad-submit 命令测试用 cmd + mock server。
// comment 为空时触发 stdin 读取路径。
func makeSelfEvalGradSubmitStdinTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/addSelfGradEvaluation":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "grad-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("comment", "", "")
	return cmd
}

// TestSelfEvalGradSubmitCmd_EmptyStdin_PrintsError 锁定 self-eval P2-5（19 轮审计）：
// grad-submit 空 stdin（无输入）应输出 envelope.Error(400) 并以参数错误退出码 3，
// 与 self-eval submit 的既有契约对称（此前 grad-submit 仅 happy path 有测试）。
func TestSelfEvalGradSubmitCmd_EmptyStdin_PrintsError(t *testing.T) {
	cmd := makeSelfEvalGradSubmitStdinTestCmd(t)

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

	stdoutBuf, _, restore := captureStdio(t)
	selfEvalGradSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()

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
}

// TestSelfEvalGradSubmitCmd_StdinReadError_Propagates 锁定 grad-submit stdin 读错误
// 不被掩盖为"评价内容不能为空"：已关闭文件作 stdin 时 ReadString 返回非 EOF 错误，
// 应走 printError（exit 2）且 stderr 含真实错误消息。
func TestSelfEvalGradSubmitCmd_StdinReadError_Propagates(t *testing.T) {
	cmd := makeSelfEvalGradSubmitStdinTestCmd(t)

	// 创建一个已关闭的文件替换 stdin
	f, err := os.CreateTemp("", "grad-stdin-closed-*")
	if err != nil {
		t.Fatalf("CreateTemp 失败: %v", err)
	}
	_ = f.Close()
	_ = os.Remove(f.Name())

	origStdin := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = origStdin }()

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalGradSubmitCmd.Run(cmd, nil)
	restore()
	stderr := stderrBuf.String()

	// stdin I/O 错误走 printError → pendingExitCode=2（服务端/网络档）
	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("stdin 读错误应触发 pendingExitCode=2，实际 %d", got)
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
