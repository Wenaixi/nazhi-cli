package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCircleWrite_InvalidID_RejectsWithoutRequest 锁定 circle 三命令对非法
// --id 的零请求行为：--id 空串/非正数/非数字时以参数错误拒绝（400/exit 3），
// 且不发出任何业务请求。修复前三个命令先 buildBizClient 再校验，--id 非法
// 时（token 合法）会真的向服务端发请求——脚本以为是参数错误却在打真接口。
func TestCircleWrite_InvalidID_RejectsWithoutRequest(t *testing.T) {
	cases := []struct {
		name string
		run  func(*cobra.Command, []string)
		id   string
		want string
	}{
		{"comment-empty", circleCommentCmd.Run, "", "--id 为必填"},
		{"comment-negative", circleCommentCmd.Run, "-5", "--id 必须为正整数"},
		{"comment-nonnumeric", circleCommentCmd.Run, "abc", "--id 必须为正整数"},
		{"like-empty", circleLikeCmd.Run, "", "--id 为必填"},
		{"like-negative", circleLikeCmd.Run, "-5", "--id 必须为正整数"},
		{"delete-empty", circleDeleteCmd.Run, "", "--id 为必填"},
		{"delete-negative", circleDeleteCmd.Run, "-5", "--id 必须为正整数"},
		{"delete-nonnumeric", circleDeleteCmd.Run, "abc", "--id 必须为正整数"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requestHit := false

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestHit = true
				_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			}))
			defer server.Close()

			cmd := &cobra.Command{Use: "circle-write"}
			cmd.SetContext(t.Context())
			cmd.Flags().String("id", "", "")
			cmd.Flags().String("content", "", "")
			cmd.Flags().String("token", "", "")
			_ = cmd.Flags().Set("token", "test-token")
			cmd.Flags().String("base-url", "", "")
			_ = cmd.Flags().Set("base-url", server.URL)
			cmd.Flags().Int("timeout", 5, "")
			cmd.Flags().Bool("verbose", false, "")
			cmd.Flags().Bool("quiet", false, "")

			originalQuiet, originalVerbose := quiet, verbose
			quiet, verbose = false, false
			pendingExitCode.Store(0)
			t.Cleanup(func() {
				quiet, verbose = originalQuiet, originalVerbose
				pendingExitCode.Store(0)
				_ = closeAllClients()
			})

			_ = cmd.Flags().Set("id", tc.id)

			stdout, _, restore := captureStdio(t)
			tc.run(cmd, nil)
			restore()

			if requestHit {
				t.Fatal("非法 --id 不应发出任何业务请求")
			}
			if pendingExitCode.Load() != 3 {
				t.Errorf("非法 --id 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Errorf("应输出 %q 参数错误 envelope，实际: %q", tc.want, stdout.String())
			}
		})
	}
}
