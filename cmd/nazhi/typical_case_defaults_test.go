package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestTypicalCaseList_DefaultPageSizeMatchesFrontend(t *testing.T) {
	flag := typicalCaseListCmd.Flags().Lookup("page-size")
	if flag == nil {
		t.Fatal("典型案例列表应注册 page-size 参数")
	}
	if flag.DefValue != "10" {
		t.Fatalf("前端默认 pageSize=10，CLI 实际默认 %s", flag.DefValue)
	}
}

// typical-case list 只拒 <0，--page 0 / --page-size 0 放行发出 pageNo=0。
// circle_metadata.go:83-89 同形状参数要求 >0，此处对齐为 ≤0 拒绝（400/exit3）。
func TestTypicalCaseList_PageZeroRejected(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string]string
	}{
		{"page 0", map[string]string{"page": "0"}},
		{"page-size 0", map[string]string{"page-size": "0"}},
		{"both 0", map[string]string{"page": "0", "page-size": "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "typical-case-list"}
			cmd.SetContext(context.Background())
			cmd.Flags().Int("page", 1, "")
			cmd.Flags().Int("page-size", 10, "")
			for k, v := range tc.flags {
				_ = cmd.Flags().Set(k, v)
			}
			cmd.Flags().Int("status", 3, "")
			cmd.Flags().String("token", "", "")
			_ = cmd.Flags().Set("token", "test-token")
			cmd.Flags().String("base-url", "", "")
			_ = cmd.Flags().Set("base-url", "http://127.0.0.1:1")
			cmd.Flags().Int("timeout", 5, "")

			originalQuiet, originalVerbose := quiet, verbose
			quiet, verbose = false, false
			pendingExitCode.Store(0)
			t.Cleanup(func() {
				quiet, verbose = originalQuiet, originalVerbose
				pendingExitCode.Store(0)
			})

			stdout, _, restore := captureStdio(t)
			typicalCaseListCmd.Run(cmd, nil)
			restore()

			if pendingExitCode.Load() != 3 {
				t.Errorf("page/page-size=0 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
			}
			if !strings.Contains(stdout.String(), `"code": 400`) {
				t.Errorf("应输出 400 参数错误 envelope，实际: %s", stdout.String())
			}
		})
	}
}
