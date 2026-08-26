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
func makeTaskSubmitTestCmd(t *testing.T, payloadRaw string) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"测试学校","className":"高一(8)班","seat":45}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null,"dataList":null,"dataMap":{"task_name":"测试任务","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"普通任务说明","type":10},"pageBean":null}`))
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
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestTaskSubmitCmd_InvalidPayloadJSON_Exit3 payload JSON 非法时 exit 3（参数错误）。
func TestTaskSubmitCmd_InvalidPayloadJSON_Exit3(t *testing.T) {
	cmd := makeTaskSubmitTestCmd(t, `{not-json`)
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

	quiet = false
	pendingExitCode.Store(0)

	_, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("非法 payload JSON 应 pendingExitCode=3，实际 %d；stderr=%q", got, stderr)
	}
	if !strings.Contains(stderr, `"code": 400`) {
		t.Errorf("stderr 应含 code=400，实际: %q", stderr)
	}
}

// TestTaskSubmitCmd_TopLevelNull_Exit3 验证顶层 null payload 在解码前被拒绝。
func TestTaskSubmitCmd_TopLevelNull_Exit3(t *testing.T) {
	cmd := makeTaskSubmitTestCmd(t, "null")
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

	quiet = false
	pendingExitCode.Store(0)

	_, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()

	if got := pendingExitCode.Load(); got != 3 {
		t.Fatalf("顶层 null 应 pendingExitCode=3，实际 %d；stderr=%q", got, stderrBuf.String())
	}
	if !strings.Contains(stderrBuf.String(), "顶层 JSON 必须为对象") {
		t.Fatalf("stderr 应说明顶层 JSON 必须为对象，实际=%q", stderrBuf.String())
	}
}

// TestTaskSubmitCmd_WithPayload 验证新最小输入模型能正确提交并输出 envelope。
func TestTaskSubmitCmd_WithPayload(t *testing.T) {
	payload := `{"taskId":1001,"content":"测试任务心得","address":"高一(8)班"}`
	cmd := makeTaskSubmitTestCmd(t, payload)
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

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
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"code": 1`) {
		t.Errorf("stdout 应包含 code: 1（业务响应码），实际: %q", stdout)
	}
}

// TestTaskSubmitCmd_MissingPayload_PrintsError 验证 --payload 缺省时输出 envelope.Error。
func TestTaskSubmitCmd_MissingPayload_PrintsError(t *testing.T) {
	cmd := makeTaskSubmitTestCmd(t, "")
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("缺 payload 应触发 pendingExitCode=3（envelope.Error(400)），实际 %d", got)
	}
	if !strings.Contains(stdout, `"status": "error"`) {
		t.Errorf("stdout 应包含 envelope.Error，实际: %q", stdout)
	}
	if !strings.Contains(stdout, "payload") {
		t.Errorf("stdout 应包含 payload 提示，实际: %q", stdout)
	}
	_ = stderr
}

// TestTaskSubmitCmd_FilePayload 验证 @file.json 语法从文件读取最小输入模型。
func TestTaskSubmitCmd_FilePayload(t *testing.T) {
	payloadContent := `{"taskId":1001,"content":"文件测试任务心得"}`
	payloadPath := t.TempDir() + "/task.json"
	if err := os.WriteFile(payloadPath, []byte(payloadContent), 0644); err != nil {
		t.Fatalf("写入 payload 文件失败: %v", err)
	}

	cmd := makeTaskSubmitTestCmd(t, "@"+payloadPath)
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

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
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
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
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"测试学校","className":"高一(8)班","seat":45}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null,"dataList":null,"dataMap":{"task_name":"测试任务","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"普通任务说明","type":10},"pageBean":null}`))
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
	cmd.Flags().String("payload", `{"taskId":1001,"content":"测试任务心得"}`, "")
	_ = cmd.Flags().Set("payload", `{"taskId":1001,"content":"测试任务心得"}`)
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("业务拒绝（ErrBusinessRejected）应触发 pendingExitCode=1，实际 %d", got)
	}
	if !strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 应包含 error envelope，实际: %q", stderr)
	}
	_ = stdout
}
