package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// TestTaskList_PartialFailure_OutputsEnvelope 验证 F9 修复：
// 5/8 维度业务失败时，FetchTasks 返回 (tasks, ErrBusinessRejected)，
// taskListCmd 必须输出 {status:partial, tasks, error} envelope，
// 不走 printError（stderr 不应含 error JSON），同时 pendingExitCode=1。
//
// 历史 bug：err != nil 一律走 printError → return，stdout 空，
// 下游拿不到任何成功维度的任务数据。
func TestTaskList_PartialFailure_OutputsEnvelope(t *testing.T) {
	const dimCount = 3
	const failDimID = "2"

	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B"}, // 业务错误（code != 1）
		{"id": 3, "name": "成功C"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu", "/api/studentInfo/getMyInfo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
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
				// 模拟业务错误（code=500 + msg），触发 ErrBusinessRejected 包装路径
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
		client.WithSSOBase(srv.URL),
		client.WithToken("test-token"),
	)
	trackClient(c)
	t.Cleanup(func() { _ = c.Close() })

	cmd := &cobra.Command{Use: "task-list"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "test-token", "")
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")

	// 抑制 quiet 防止 printError 吞 stderr
	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskListCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码必须标记为 1（partial failure 是失败信号）
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("partial failure 应触发 pendingExitCode=1，实际 %d", got)
	}

	// stderr 不应含 error JSON（F9：partial failure 走 envelope，不走 printError）
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应含 error JSON（partial failure 应走 envelope），实际: %q", stderr)
	}

	// stdout 应包含 status: partial envelope
	if !strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("stdout 应包含 status: partial，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"reason": "fetch_tasks_partial_failure"`) {
		t.Errorf("stdout 应包含 reason: fetch_tasks_partial_failure，实际: %q", stdout)
	}

	// 关键断言：成功维度的 tasks 数据必须仍输出到 stdout
	if !strings.Contains(stdout, `"tasks"`) {
		t.Errorf("stdout 应包含 tasks 字段（成功维度的数据），实际: %q", stdout)
	}
	// 任务 ID 2000 应出现（来自成功维度 1 和 3）
	if !strings.Contains(stdout, `"id": 2000`) {
		t.Errorf("stdout 应包含成功维度的任务 id 2000，实际: %q", stdout)
	}
	// 失败维度的错误信息应在 error 字段
	if !strings.Contains(stdout, "维度 2") {
		t.Errorf("stdout error 字段应包含失败维度 2 的信息，实际: %q", stdout)
	}
}

// TestTaskList_AllFailure_StillPrintsError 验证 F9 边界：全失败场景下
// 仍走 printError 路径（不是 partial 路径），符合「全维度无成功数据 → 无法输出 envelope」的语义。
func TestTaskList_AllFailure_StillPrintsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataList":[{"id":1,"name":"A"},{"id":2,"name":"B"}]}`))
		case "/api/studentCircleNew/getCircleStatistics":
			// 全部失败
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":500,"msg":"服务不可用"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithSSOBase(srv.URL),
		client.WithToken("test-token"),
	)
	trackClient(c)
	t.Cleanup(func() { _ = c.Close() })

	cmd := &cobra.Command{Use: "task-list"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "test-token", "")
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")

	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskListCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码标记为 1
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("全失败应触发 pendingExitCode=1，实际 %d", got)
	}
	// 走 printError，stderr 含 error JSON
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("全失败应走 printError 路径，stderr 应含 error JSON，实际 stderr: %q", stderr)
	}
	// stdout 不应有 partial envelope
	if strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("全失败不应输出 partial envelope，实际 stdout: %q", stdout)
	}
}

// 静默：避免 import 错误
var _ = bytes.NewBuffer
var _ = io.Copy
var _ = atomic.Int32{}
var _ = time.Second
var _ = os.Stderr
