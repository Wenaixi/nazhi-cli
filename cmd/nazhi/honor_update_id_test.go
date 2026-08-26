package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHonorUpdate_MissingID_RejectsWithoutRequest 锁定 cmd/nazhi/honor.go:170-199
// 当前 Run 函数对 payload["id"] 零校验，会发出无 id 的业务请求。
// 前端 performanceM.vue:489 编辑提交必然注入记录 id，此处对齐该契约。
// 修复后：payload 缺 id 或 id 非正数时以参数错误拒绝（400/exit3）且不发业务请求。
func TestHonorUpdate_MissingID_RejectsWithoutRequest(t *testing.T) {
	requestHit := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "updateHonor") {
			requestHit = true
		}
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeCapabilityPayloadTestCmd(t, server.URL, `{"typeId":1147,"level":5}`)
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
	honorUpdateCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("payload 缺 id 时不应发出业务请求")
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("缺 id 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
	}
	if !strings.Contains(stdout.String(), `"code": 400`) {
		t.Errorf("应输出 400 参数错误 envelope，实际: %s", stdout.String())
	}
}
