package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ─── task submit / task edit payload 未知顶层键必须拒绝 ───

// makeTaskPayloadTestCmd 创建带通用业务参数和 payload 的 task submit/edit 测试命令实例。
func makeTaskPayloadTestCmd(t *testing.T, baseURL, payload string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "task-payload-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	_ = cmd.Flags().Set("payload", payload)
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")
	return cmd
}

// TestTaskSubmit_UnknownTopLevelKey_Rejects 锁定
// task submit payload 未知顶层键（如 imagePath 单数拼错）此前被静默忽略，
// 图片不上传仍照常发请求。对齐 user update：未知键以参数错误拒绝（400/exit3）。
func TestTaskSubmit_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getStudentTask") || strings.Contains(r.URL.Path, "addCircle") {
			requestHit = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeTaskPayloadTestCmd(t, server.URL, `{"taskId":18154,"content":"内容","imagePath":"./photo.jpg"}`)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("task submit 含未知键时不应发出业务请求")
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
	}
	if !strings.Contains(stdout.String()+stderr.String(), `"code": 400`) {
		t.Errorf("应输出 400 参数错误 envelope，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String()+stderr.String(), "未知键") {
		t.Errorf("应含未知键提示，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

// TestTaskEdit_UnknownTopLevelKey_Rejects 锁定：task edit 同款拒绝。
func TestTaskEdit_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getStudentTask") || strings.Contains(r.URL.Path, "editCircle") {
			requestHit = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeTaskPayloadTestCmd(t, server.URL, `{"id":5464109,"taskId":18151,"content":"修改","imagePath":"./photo.jpg"}`)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	taskEditCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("task edit 含未知键时不应发出业务请求")
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
	}
	if !strings.Contains(stdout.String()+stderr.String(), `"code": 400`) {
		t.Errorf("应输出 400 参数错误 envelope，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String()+stderr.String(), "未知键") {
		t.Errorf("应含未知键提示，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

// ─── honor update / typical-case update payload 未知顶层键必须拒绝 ───

// TestHonorUpdate_UnknownTopLevelKey_Rejects 锁定
// honor update map payload 未知键静默透传服务端：拼错键名→服务端忽略→204 成功但零修改。
// 对齐 user update：未知键以参数错误拒绝（400/exit3），不发业务请求。
func TestHonorUpdate_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "updateHonor") {
			requestHit = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeCapabilityPayloadTestCmd(t, server.URL, `{"id":56241,"typeId":1147,"level":5,"evaluationAgencyX":"示例中学"}`)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	honorUpdateCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("honor update 含未知键时不应发出业务请求")
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
	}
	if !strings.Contains(stdout.String(), `"code": 400`) && !strings.Contains(stderr.String(), `"code": 400`) {
		t.Errorf("应输出 400 参数错误 envelope，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "未知键") && !strings.Contains(stderr.String(), "未知键") {
		t.Errorf("应含未知键提示，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

// TestTypicalCaseUpdate_UnknownTopLevelKey_Rejects 锁定：typical-case update 同款拒绝。
func TestTypicalCaseUpdate_UnknownTopLevelKey_Rejects(t *testing.T) {
	requestHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "updateTypicalCase") {
			requestHit = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeCapabilityPayloadTestCmd(t, server.URL, `{"id":56241,"title":"标题","titel":"拼错的标题"}`)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	typicalCaseUpdateCmd.Run(cmd, nil)
	restore()

	if requestHit {
		t.Fatal("typical-case update 含未知键时不应发出业务请求")
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("未知键应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
	}
	if !strings.Contains(stdout.String(), `"code": 400`) && !strings.Contains(stderr.String(), `"code": 400`) {
		t.Errorf("应输出 400 参数错误 envelope，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "未知键") && !strings.Contains(stderr.String(), "未知键") {
		t.Errorf("应含未知键提示，实际 stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}
