package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHonorDropdownCommandsEmptyDataOutputsEmptyArray 回归测试：
// 服务端 dataList 为 null 时，honor type-options / level-options / levels
// 的 envelope.data 应为 [] 而非 null。
//
// 背景（十三域审计 P2-M）：同仓五个 circle 元数据命令均有 nil→[] 归一，
// 荣誉三个下拉命令把 Go nil slice 直塞 Success 信封输出 "data":null——
// jq '.data[]' 对 null 报错退出，同类命令却可正常管道消费，脚本作者无从预期。
func TestHonorDropdownCommandsEmptyDataOutputsEmptyArray(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		apiPath string
		setType bool
	}{
		{name: "type-options", cmd: honorTypeOptionsCmd, apiPath: "/api/studentMoralEduNew/getHonorTypeForSelect"},
		{name: "level-options", cmd: honorLevelOptionsCmd, apiPath: "/api/studentMoralEduNew/getHonorTypeForSelect"},
		{name: "levels", cmd: honorLevelsCmd, apiPath: "/api/studentMoralEduNew/getHonorLevel", setType: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/", "/api/studentInfo/getMenu":
					_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
				case "/api/studentInfo/getMyInfo":
					_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
				case tt.apiPath:
					_, _ = w.Write([]byte(`{"code":1,"dataList":null}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			cmd := &cobra.Command{Use: "honor-" + tt.name}
			cmd.SetContext(context.Background())
			cmd.Flags().String("token", "", "")
			cmd.Flags().String("base-url", "", "")
			cmd.Flags().Int("timeout", 5, "")
			if tt.setType {
				cmd.Flags().Int64("type-id", 0, "")
			}
			_ = cmd.Flags().Set("token", "test-token")
			_ = cmd.Flags().Set("base-url", srv.URL)
			if tt.setType {
				_ = cmd.Flags().Set("type-id", "1147")
			}

			originalQuiet, originalVerbose := quiet, verbose
			quiet = false
			verbose = false
			pendingExitCode.Store(0)
			t.Cleanup(func() {
				quiet, verbose = originalQuiet, originalVerbose
				pendingExitCode.Store(0)
				_ = closeAllClients()
			})

			stdout, stderr, restore := captureStdio(t)
			tt.cmd.Run(cmd, nil)
			restore()

			if pendingExitCode.Load() != 0 {
				t.Fatalf("%s 空数据不应报错，实际 exit=%d；stderr=%s", tt.name, pendingExitCode.Load(), stderr.String())
			}
			out := stdout.String()
			if strings.Contains(out, "\"data\": null") || strings.Contains(out, "\"data\":null") {
				t.Fatalf("%s 空数据应归一为 [] 而非 null，实际: %s", tt.name, out)
			}
		})
	}
}
