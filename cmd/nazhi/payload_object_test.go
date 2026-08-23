package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func TestPayloadCommandsRejectTopLevelNull(t *testing.T) {
	tests := []struct {
		name string
		run  func(*cobra.Command, []string)
	}{
		{name: "self-eval submit", run: selfEvalSubmitCmd.Run},
		{name: "honor add", run: honorAddCmd.Run},
		{name: "typical-case submit", run: typicalCaseSubmitCmd.Run},
		{name: "typical-case update", run: typicalCaseUpdateCmd.Run},
		{name: "user update", run: userUpdateCmd.Run},
		{name: "task submit", run: taskSubmitCmd.Run},
		{name: "task edit", run: taskEditCmd.Run},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/", "/api/studentInfo/getMenu":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
				case "/api/studentInfo/getMyInfo":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
				default:
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"code":500,"msg":"unexpected request"}`))
				}
			}))
			t.Cleanup(srv.Close)

			cmd := makeNullPayloadTestCmd(t, srv.URL)
			quiet = false
			verbose = false
			pendingExitCode.Store(0)
			t.Cleanup(func() { _ = closeAllClients() })

			stdoutBuf, stderrBuf, restore := captureStdio(t)
			tt.run(cmd, nil)
			restore()

			if got := pendingExitCode.Load(); got != 3 {
				t.Fatalf("顶层 null 应触发 pendingExitCode=3，实际 %d；stdout=%q stderr=%q", got, stdoutBuf.String(), stderrBuf.String())
			}
			if !strings.Contains(stderrBuf.String(), `"code": 400`) {
				t.Errorf("顶层 null 应走参数错误 envelope，实际 stderr=%q", stderrBuf.String())
			}
			if !strings.Contains(stderrBuf.String(), "顶层 JSON 必须为对象") {
				t.Errorf("参数错误应说明顶层 JSON 必须为对象，实际 stderr=%q", stderrBuf.String())
			}
			if got := requests.Load(); got != 0 {
				t.Errorf("顶层 null 不应发业务请求，实际请求数 %d", got)
			}
		})
	}
}

func makeNullPayloadTestCmd(t *testing.T, baseURL string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "payload-null"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	_ = cmd.Flags().Set("payload", "null")
	cmd.Flags().String("comment", "", "")
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")
	return cmd
}
