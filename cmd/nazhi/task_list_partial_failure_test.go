package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// TestTaskList_PartialFailure_OutputsEnvelope 验证 F9 修复：
// 5/8 维度业务失败时，FetchTasks 返回 (tasks, ErrBusinessRejected)，
// taskListCmd 必须输出 {status:partial, tasks, error} envelope，
// 不走 printError（stderr 不应含 error JSON），同时 pendingExitCode=1。
func TestTaskList_PartialFailure_OutputsEnvelope(t *testing.T) {
	const dimCount = 3
	const failDimID = "2"

	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B"},
		{"id": 3, "name": "成功C"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			// F11 修复（group-F round-8）：getMyInfo 必须返回真实 user info，
			// 否则 session 预热步骤 4 触发 ErrEmptyUserInfo → FetchTasks 在
			// 拉取维度之前就失败，永远走不到 partial failure 分支。
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"福清一中","className":"高一八班","seat":45}}`))
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			body, _ := json.Marshal(map[string]any{
				"code": 1, "msg": "成功",
				"dataList": dims,
			})
			_, _ = w.Write(body)
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			if dimID == failDimID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":500,"msg":"维度服务暂时不可用"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[{"id":2000,"name":"任务` + dimID + `","circleTypeId":9999,"hours":1.0,"circleTaskStatus":"未提交","upPic":1}]}`))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5),
	)
	client.Track(c)

	cmd := &cobra.Command{}
	taskListCmd.Run(cmd, []string{
		"--token", "test-token",
		"--base-url", srv.URL,
		"--timeout", "5",
	})
	// cobra 默认不触发 Run 回调，这里修改 flags 然后手动调 Run
	cmd.Flags().String("token", "test-token", "")
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")

	quiet = false
	pendingExitCode.Store(0)

	var stdoutBuf, stderrBuf strings.Builder
	origPrintJSON := printJSON
	origPrintVerbose := printVerbose
	defer func() {
		printJSON = origPrintJSON
		printVerbose = origPrintVerbose
	}()
	printJSON = func(v any) {
		b, _ := json.Marshal(v)
		_, _ = stdoutBuf.Write(b)
	}
	printVerbose = func(format string, args ...any) {}
	origPrintError := printError
	defer func() { printError = origPrintError }()
	printError = func(err error) {
		b, _ := json.Marshal(map[string]any{"error": true, "message": err.Error()})
		_, _ = stderrBuf.Write(b)
	}

	taskListCmd.Run(cmd, []string{})

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("partial failure 应触发 pendingExitCode=1，实际 %d", got)
	}

	if strings.Contains(stderr, "error") {
		t.Errorf("stderr 不应含 error JSON（partial failure 应走 envelope），实际: %q", stderr)
	}
	if !strings.Contains(stdout, "partial") || !strings.Contains(stdout, "fetch_tasks_partial_failure") {
		t.Errorf("stdout 应包含 status: partial 和 reason，实际: %s", stdout)
	}
	if !strings.Contains(stdout, `"tasks"`) {
		t.Errorf("stdout 应包含 tasks 字段（成功维度的数据），实际: %s", stdout)
	}
	if !strings.Contains(stdout, `2000`) {
		t.Errorf("stdout 应包含成功维度的任务 id 2000，实际: %s", stdout)
	}
	if !strings.Contains(stdout, "维度服务暂时不可用") {
		t.Errorf("stdout error 字段应包含失败维度 2 的信息，实际: %s", stdout)
	}
}
