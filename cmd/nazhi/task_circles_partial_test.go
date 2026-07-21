package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeCirclesPartialServer 模拟多页拉取：第 1 页成功，第 2 页业务失败。
// TotalNum=1000、TotalPage=2，pageSize 默认 500，触发第 2 页请求。
func makeCirclesPartialServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"示例中学","className":"高一(8)班","seat":45}}`))
		case "/api/studentCircleNew/getStudentCircle":
			pageNo := r.URL.Query().Get("pageNo")
			if pageNo == "1" || pageNo == "" {
				w.WriteHeader(http.StatusOK)
				items := make([]map[string]any, 0, 2)
				for i := 1; i <= 2; i++ {
					items = append(items, map[string]any{"id": 1000 + i, "content": "page1"})
				}
				body, _ := json.Marshal(map[string]any{
					"code": 1, "msg": "成功",
					"dataList": items,
					"pageBean": map[string]any{
						"pageNo": 1, "pageSize": 500, "totalPage": 2, "totalNum": 1000,
					},
				})
				_, _ = w.Write(body)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":500,"msg":"第2页服务暂时不可用"}`))
		default:
			t.Errorf("未预期请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func makeCirclesBizCmd(t *testing.T, srvURL string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "task-circles"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srvURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Bool("count", false, "")
	cmd.Flags().Int("offset", 0, "")
	cmd.Flags().Int("limit", 0, "")
	return cmd
}

// TestTaskSubmitted_PartialPageFailure_OutputsPartialEnvelope
// GetSubmittedCirclesJSON 部分页失败时：err!=nil 且 raw 非空 → envelope.Partial(207,...)，
// 不得走 printError 丢弃已拉到的数据。对齐 task list partial 语义。
func TestTaskSubmitted_PartialPageFailure_OutputsPartialEnvelope(t *testing.T) {
	srv := makeCirclesPartialServer(t)
	defer srv.Close()

	cmd := makeCirclesBizCmd(t, srv.URL)
	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskSubmittedCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("partial 应 pendingExitCode=1，实际 %d；stderr=%q stdout=%q", got, stderr, stdout)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("partial 不应走 printError，stderr 不应含 error envelope，实际: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("stdout 应含 status=partial，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"code": 207`) {
		t.Errorf("stdout 应含 code=207，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"id": 1001`) && !strings.Contains(stdout, `"id":1001`) {
		t.Errorf("stdout 应保留第 1 页记录 id，实际: %q", stdout)
	}
}

func TestTaskTeacher_PartialPageFailure_OutputsPartialEnvelope(t *testing.T) {
	srv := makeCirclesPartialServer(t)
	defer srv.Close()
	cmd := makeCirclesBizCmd(t, srv.URL)
	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskTeacherCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("partial 应 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("partial 不应走 printError，实际 stderr: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("stdout 应含 status=partial，实际: %q", stdout)
	}
}

func TestTaskPublic_PartialPageFailure_OutputsPartialEnvelope(t *testing.T) {
	srv := makeCirclesPartialServer(t)
	defer srv.Close()
	cmd := makeCirclesBizCmd(t, srv.URL)
	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskPublicCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("partial 应 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("partial 不应走 printError，实际 stderr: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("stdout 应含 status=partial，实际: %q", stdout)
	}
}

func TestTaskWithdrawn_PartialPageFailure_OutputsPartialEnvelope(t *testing.T) {
	srv := makeCirclesPartialServer(t)
	defer srv.Close()
	cmd := makeCirclesBizCmd(t, srv.URL)
	quiet = false
	pendingExitCode.Store(0)
	stdoutBuf, stderrBuf, restore := captureStdio(t)
	taskWithdrawnCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("partial 应 pendingExitCode=1，实际 %d", got)
	}
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("partial 不应走 printError，实际 stderr: %q", stderr)
	}
	if !strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("stdout 应含 status=partial，实际: %q", stdout)
	}
}
