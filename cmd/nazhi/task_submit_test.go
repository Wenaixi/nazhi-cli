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

// makeTaskSubmitTestCmd 创建 task submit 命令的测试用 cobra.Command + mock biz server。
// payloadRaw 是 --payload flag 的值（空字符串时不设 flag，用于测试缺省场景）。
func makeTaskSubmitTestCmd(t *testing.T, payloadRaw string) (*cobra.Command, *httptest.Server) {
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
		case "/api/studentCircleNew/addCircle":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":5}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":500,"msg":"未知路径"}`))
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "task-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("payload", "", "")
	if payloadRaw != "" {
		_ = cmd.Flags().Set("payload", payloadRaw)
	}
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")
	return cmd, srv
}

// TestTaskSubmitCmd_WithPayload 验证 --payload flag 正确传递并输出提交结果 JSON。
func TestTaskSubmitCmd_WithPayload(t *testing.T) {
	payload := `{"circleTaskId":1001,"circleTypeId":9256,"name":"测试任务","hours":1}`
	cmd, _ := makeTaskSubmitTestCmd(t, payload)

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("正常路径不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"code": 1`) {
		t.Errorf("stdout 应包含 code: 1，实际: %q", stdout)
	}
}

// TestTaskSubmitCmd_MissingPayload_PrintsError 验证 --payload 缺省时输出 error。
func TestTaskSubmitCmd_MissingPayload_PrintsError(t *testing.T) {
	cmd, _ := makeTaskSubmitTestCmd(t, "") // 不设 payload flag

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("缺 payload 应触发 pendingExitCode=1，实际 %d", got)
	}
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stderr, "payload") {
		t.Errorf("stderr 应包含 payload 提示，实际: %q", stderr)
	}
	_ = stdout
}

// TestTaskSubmitCmd_FilePayload 验证 @file.json 语法从文件读取 payload。
func TestTaskSubmitCmd_FilePayload(t *testing.T) {
	payloadContent := `{"circleTaskId":2002,"circleTypeId":9256,"name":"文件测试任务","hours":2}`
	payloadPath := t.TempDir() + "/task.json"
	if err := os.WriteFile(payloadPath, []byte(payloadContent), 0644); err != nil {
		t.Fatalf("写入 payload 文件失败: %v", err)
	}

	cmd, _ := makeTaskSubmitTestCmd(t, "@"+payloadPath)

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("@file 路径不应触发 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"code": 1`) {
		t.Errorf("stdout 应包含 code: 1，实际: %q", stdout)
	}
}

// TestTaskSubmitCmd_ServerError 验证服务端返回业务错误时传播。
func TestTaskSubmitCmd_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"测试学校","className":"高一八班","seat":45}}`))
		case "/api/studentCircleNew/addCircle":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":500,"msg":"业务处理失败"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cmd := &cobra.Command{Use: "task-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("payload", `{"circleTaskId":1001,"circleTypeId":9256,"name":"测试任务","hours":1}`, "")
	_ = cmd.Flags().Set("payload", `{"circleTaskId":1001,"circleTypeId":9256,"name":"测试任务","hours":1}`)
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
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
