package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// I3-01：task preview 与 task submit/edit 同族入口必须共用未知键白名单。
// 现状：preview 仅 parse+decode 直接 json.Unmarshal——拼错键（如 imagePath 单数）
// 在 preview 静默丢弃、task submit/edit 却 400 拒绝，两套契约。
// 修复：preview 解码前复用 unknownTaskInputKeys(payloadBytes)，未知键以参数错误
// 拒绝（400/exit3），且不发任何业务请求（含预览必拉的 getCircleTypeByTaskId 元数据）。

// makeTaskPreviewTestCmd 创建带通用业务参数和 payload 的 task preview 测试命令实例。
func makeTaskPreviewTestCmd(t *testing.T, baseURL, payload string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "task-preview-test"}
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
	cmd.Flags().Bool("edit", false, "")
	return cmd
}

// TestTaskPreview_UnknownTopLevelKey_Rejects 锁定 I3-01：task preview 含未知顶层键
// （如 imagePath 单数拼错）必须以参数错误拒绝（400/exit3），且不发任何业务请求。
// 审查要求：与 submit/edit 共存时同族入口不能两套契约——preview 静默丢弃会让用户
// 在真正 submit 时才被 400 拒绝，拼错键的 payload 在预览阶段得不到任何预警。
func TestTaskPreview_UnknownTopLevelKey_Rejects(t *testing.T) {
	type metaHit struct{ hit bool }
	var m metaHit
	requestHit := &m
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 预热路径也必须响应，否则 preview 的 buildBizClient 阶段就会失败——这里
		// 记录所有非预热 path，任何业务/元数据请求都说明守卫失效。
		if r.URL.Path == "/" || r.URL.Path == "/api/studentInfo/getMenu" ||
			r.URL.Path == "/api/studentInfo/getMyInfo" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		requestHit.hit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeTaskPreviewTestCmd(t, server.URL, `{"taskId":18154,"content":"内容","imagePath":"./photo.jpg"}`)
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	taskPreviewCmd.Run(cmd, nil)
	restore()

	if requestHit.hit {
		t.Fatal("task preview 含未知键时不应发出任何业务/元数据请求")
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

// TestTaskPreviewEdit_UnknownTopLevelKey_Rejects 锁定 I3-01：task preview --edit 同款拒绝。
func TestTaskPreviewEdit_UnknownTopLevelKey_Rejects(t *testing.T) {
	type metaHit struct{ hit bool }
	var m metaHit
	requestHit := &m
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/api/studentInfo/getMenu" ||
			r.URL.Path == "/api/studentInfo/getMyInfo" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		requestHit.hit = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	}))
	defer server.Close()

	cmd := makeTaskPreviewTestCmd(t, server.URL, `{"id":5464109,"taskId":18151,"content":"修改","imagePath":"./photo.jpg"}`)
	_ = cmd.Flags().Set("edit", "true")
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdout, stderr, restore := captureStdio(t)
	taskPreviewCmd.Run(cmd, nil)
	restore()

	if requestHit.hit {
		t.Fatal("task preview --edit 含未知键时不应发出任何业务/元数据请求")
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