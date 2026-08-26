package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHonorDelete_NegativeID_RejectsWithoutRequest 锁定 cmd/nazhi/honor.go:149-155
// 当前 Run 函数对 --id 只判 == 0，负值 -5 会放行发出 deleteHonorById?id=-5 请求。
// 同域纪律：honor levels 用 typeID <= 0、typical-case delete 用 id <= 0，
// honor delete 是 honor/typical-case 两域唯一漏负数的 id 入口（19 轮审计 P2-1）。
// 修复后：--id 非正数时以参数错误拒绝（400/exit3）且不发业务请求。
func TestHonorDelete_NegativeID_RejectsWithoutRequest(t *testing.T) {
	requestHit := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "deleteHonorById") {
			requestHit = true
		}
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := &cobra.Command{Use: "honor-delete"}
	cmd.SetContext(t.Context())
	cmd.Flags().Int64("id", 0, "")
	_ = cmd.Flags().Set("id", "-5")
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", server.URL)
	cmd.Flags().Int("timeout", 5, "")

	originalQuiet, originalVerbose := quiet, verbose
	quiet = false
	verbose = false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, _, restore := captureStdio(t)
	honorDeleteCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("--id 为负数时不应发出业务请求")
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("负 id 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
	}
	if !strings.Contains(stdout.String(), `"code": 400`) {
		t.Errorf("应输出 400 参数错误 envelope，实际: %s", stdout.String())
	}
}
