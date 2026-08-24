package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newLoneOffsetServer 空服务器：任何到达的请求都判失败——
// 单独 --offset 被拒绝时不应发出任何网络请求。
func newLoneOffsetServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("单独 --offset 拒绝路径不应发请求, 收到: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
}

// TestTaskCirclesCommands_RejectLoneOffset teacher/public/withdrawn 三命令
// 单独 --offset 必须以参数错误（退出码 3）拒绝，与 task submitted/done 的既有
// 守卫行为对齐；此前静默返回全量数据会让分页脚本以成功状态拿到错误形状。
func TestTaskCirclesCommands_RejectLoneOffset(t *testing.T) {
	cases := []struct {
		name string
		run  func(*cobra.Command, []string)
	}{
		{"teacher", taskTeacherCmd.Run},
		{"public", taskPublicCmd.Run},
		{"withdrawn", taskWithdrawnCmd.Run},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newLoneOffsetServer(t)
			defer srv.Close()

			cmd := &cobra.Command{Use: tc.name}
			cmd.Flags().String("token", "", "")
			_ = cmd.Flags().Set("token", "test-token")
			cmd.Flags().String("base-url", "", "")
			_ = cmd.Flags().Set("base-url", srv.URL)
			cmd.Flags().Int("timeout", 5, "")
			cmd.Flags().Bool("count", false, "")
			cmd.Flags().Int("offset", 0, "")
			_ = cmd.Flags().Set("offset", "5")
			cmd.Flags().Int("limit", 0, "")

			quiet = false
			pendingExitCode.Store(0)
			stdoutBuf, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()
			stdout := stdoutBuf.String()

			if got := pendingExitCode.Load(); got != 3 {
				t.Errorf("单独 --offset 应拒绝为参数错误(退出码 3), 实际 %d; stdout=%q", got, stdout)
			}
			if !strings.Contains(stdout, "--offset") {
				t.Errorf("stdout 应含参数错误提示, 实际: %q", stdout)
			}
		})
	}
}
