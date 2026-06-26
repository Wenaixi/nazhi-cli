package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeSelfEvalStatusTestCmd 创建 self-eval status 命令的测试用 cobra.Command + mock server。
// bizOK 控制 session 激活路径是否成功（false 时 / 和 /getMyInfo 返回 500 模拟激活失败）。
func makeSelfEvalStatusTestCmd(t *testing.T, bizOK bool) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			if !bizOK {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			if !bizOK {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"测试学校","className":"高一八班","seat":45}}`))
		case "/api/studentMoralEduNew/querySelfEvaluation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"student_comment":"很好的学期","teacher_comment":"优秀","student_name":"张三","student_number":"TEST2025001","id":100}}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"msg":"未知路径"}`))
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "self-eval-status"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestSelfEvalStatusCmd_HappyPath 验证正常查询自我评价状态并输出 JSON。
func TestSelfEvalStatusCmd_HappyPath(t *testing.T) {
	cmd := makeSelfEvalStatusTestCmd(t, true)

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalStatusCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("正常路径不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"student_comment": "很好的学期"`) {
		t.Errorf("stdout 应包含 student_comment，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"teacher_comment": "优秀"`) {
		t.Errorf("stdout 应包含 teacher_comment，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"student_name": "张三"`) {
		t.Errorf("stdout 应包含 student_name，实际: %q", stdout)
	}
}

// TestSelfEvalStatusCmd_SessionError 验证 session 激活失败时输出 error。
func TestSelfEvalStatusCmd_SessionError(t *testing.T) {
	cmd := makeSelfEvalStatusTestCmd(t, false) // bizOK=false 模拟 session 激活失败

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	selfEvalStatusCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("session 激活失败应触发 pendingExitCode=1，实际 %d", got)
	}
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stderr, "QuerySelfEvaluation") {
		t.Errorf("stderr 应包含 QuerySelfEvaluation 信息，实际: %q", stderr)
	}
	_ = stdout
}
