package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestTaskCirclesCommands_RejectNegativeLimit 回归测试：
// 负数 --limit 必须以参数错误（退出码 3）拒绝，不发业务请求。
//
// 背景（十三域审计 P2-N）：守卫谓词 (offset>0 && limit<=0) || offset<0
// 放行 offset=0+负 limit 组合，SDK 侧 limit<=0 走全量分支——分页脚本以
// 公式计算 limit 得负时无声拿到全量数据，输出体积失控但退出码 0。
// 与正/负 offset 守卫同语义。
func TestTaskCirclesCommands_RejectNegativeLimit(t *testing.T) {
	cases := []struct {
		name string
		run  func(*cobra.Command, []string)
	}{
		{"teacher", taskTeacherCmd.Run},
		{"public", taskPublicCmd.Run},
		{"withdrawn", taskWithdrawnCmd.Run},
		{"submitted", taskSubmittedCmd.Run},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var bizCalls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bizCalls++
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/":
					_, _ = w.Write([]byte("<html>home</html>"))
				case "/api/studentInfo/getMenu":
					_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
				case "/api/studentInfo/getMyInfo":
					_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
				case "/api/studentCircleNew/getStudentCircle":
					_, _ = w.Write([]byte(`{"code":1,"dataList":[],"pageBean":{"pageNo":1,"pageSize":10,"totalNum":0,"totalPage":1}}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			cmd := &cobra.Command{Use: tc.name}
			cmd.SetContext(context.Background())
			cmd.Flags().String("token", "", "")
			_ = cmd.Flags().Set("token", "test-token")
			cmd.Flags().String("base-url", "", "")
			_ = cmd.Flags().Set("base-url", srv.URL)
			cmd.Flags().Int("timeout", 5, "")
			cmd.Flags().Bool("count", false, "")
			cmd.Flags().Int("offset", 0, "")
			cmd.Flags().Int("limit", 0, "")
			_ = cmd.Flags().Set("limit", "-3")

			quiet = false
			pendingExitCode.Store(0)
			stdoutBuf, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()
			stdout := stdoutBuf.String()

			if got := pendingExitCode.Load(); got != 3 {
				t.Errorf("负数 --limit 应拒绝为参数错误(退出码 3), 实际 %d; stdout=%q", got, stdout)
			}
			if !strings.Contains(stdout, "--limit") {
				t.Errorf("stdout 应含参数错误提示, 实际: %q", stdout)
			}
		})
	}
}
