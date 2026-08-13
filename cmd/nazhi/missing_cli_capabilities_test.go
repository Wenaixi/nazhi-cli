package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHonorUpdateCommandRegistered 验证 SDK 荣誉更新能力保留为 CLI。
func TestHonorUpdateCommandRegistered(t *testing.T) {
	if honorCmd.Commands() == nil {
		t.Fatal("honor 应注册子命令")
	}
	var found bool
	for _, command := range honorCmd.Commands() {
		if command.Name() == "update" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("honor 应注册 update 子命令")
	}
	if flag := honorUpdateCmd.Flags().Lookup("payload"); flag == nil {
		t.Fatal("honor update 应暴露 --payload")
	}
}

// TestTypicalCaseDeleteBatchCommandRegistered 验证前端批量删除能力保留为 CLI。
func TestTypicalCaseDeleteBatchCommandRegistered(t *testing.T) {
	var found bool
	for _, command := range typicalCaseCmd.Commands() {
		if command.Name() == "delete-batch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("typical-case 应注册 delete-batch 子命令")
	}
	if flag := typicalCaseDeleteBatchCmd.Flags().Lookup("payload"); flag == nil {
		t.Fatal("typical-case delete-batch 应暴露 --payload")
	}
}

// TestParseTypicalCaseBatchIDs 验证批量删除 payload 的数组边界。
func TestParseTypicalCaseBatchIDs(t *testing.T) {
	valid, err := parseTypicalCaseBatchIDs(`[1,2,3]`)
	if err != nil || len(valid) != 3 || valid[0] != 1 || valid[2] != 3 {
		t.Fatalf("合法 ID 数组解析错误: ids=%v err=%v", valid, err)
	}
	for _, raw := range []string{"null", "{}", "[]", "[0]", "[-1]", "[1.5]", "[" + `"1"` + "]"} {
		if _, err := parseTypicalCaseBatchIDs(raw); err == nil {
			t.Errorf("非法批量 ID payload 应失败: %s", raw)
		}
	}
}

// TestHonorUpdateCommandSendsObjectAndAutofillsTypeName 验证荣誉更新命令
// 保留前端对象字段，并沿用 SDK 的 typeName 自动补全。
func TestHonorUpdateCommandSendsObjectAndAutofillsTypeName(t *testing.T) {
	var gotBody map[string]any
	var updateCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/getHonorTypeForSelect":
			if r.Method != http.MethodGet {
				t.Errorf("荣誉类型反查必须使用 GET，实际 %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"label":"校三好学生","value":1147}],"returnData":[{"label":"校","value":5}]}`))
		case "/api/studentMoralEduNew/updateHonor":
			if r.Method != http.MethodPost {
				t.Errorf("荣誉更新必须使用 POST，实际 %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Errorf("解析荣誉更新请求体失败: %v", err)
			}
			updateCalled = true
			_, _ = w.Write([]byte(`{"code":1,"msg":"更新成功"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := makeCapabilityPayloadTestCmd(t, server.URL, `{"id":56241,"typeId":1147,"level":5,"evaluationAgency":"示例中学","getDate":"2026-06-30"}`)
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
	honorUpdateCmd.Run(cmd, nil)
	restore()

	if !updateCalled {
		t.Fatal("荣誉更新命令未发出业务请求")
	}
	if gotBody["id"] != float64(56241) || gotBody["typeId"] != float64(1147) {
		t.Fatalf("荣誉更新对象字段未保留: %+v", gotBody)
	}
	if gotBody["typeName"] != "校三好学生" {
		t.Fatalf("SDK 自动补全 typeName 失败，实际 %v", gotBody["typeName"])
	}
	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功命令不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "success"`) || !strings.Contains(stdout.String(), `"code": 204`) {
		t.Fatalf("荣誉更新应输出成功 envelope，实际: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), `"status": "error"`) {
		t.Fatalf("成功命令不应输出错误 envelope，实际: %s", stderr.String())
	}
}

// TestTypicalCaseDeleteBatchCommandSendsPlainIDArray 验证批量删除命令
// 将前端的纯 JSON ID 数组原样交给 SDK，而不是包装为对象。
func TestTypicalCaseDeleteBatchCommandSendsPlainIDArray(t *testing.T) {
	var gotIDs []int64
	var deleteCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentCircleNew/deleteBatchTypicalCase":
			if r.Method != http.MethodPost {
				t.Errorf("典型案例批量删除必须使用 POST，实际 %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotIDs); err != nil {
				t.Errorf("解析批量删除请求体失败: %v", err)
			}
			deleteCalled = true
			_, _ = w.Write([]byte(`{"code":1,"msg":"删除成功"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := makeCapabilityPayloadTestCmd(t, server.URL, `[1,2,3]`)
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
	typicalCaseDeleteBatchCmd.Run(cmd, nil)
	restore()

	if !deleteCalled {
		t.Fatal("典型案例批量删除命令未发出业务请求")
	}
	if len(gotIDs) != 3 || gotIDs[0] != 1 || gotIDs[1] != 2 || gotIDs[2] != 3 {
		t.Fatalf("批量删除请求体不是预期的纯 ID 数组: %v", gotIDs)
	}
	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功命令不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "success"`) || !strings.Contains(stdout.String(), `"code": 204`) {
		t.Fatalf("批量删除应输出成功 envelope，实际: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), `"status": "error"`) {
		t.Fatalf("成功命令不应输出错误 envelope，实际: %s", stderr.String())
	}
}

// makeCapabilityPayloadTestCmd 创建带通用业务参数和 payload 的命令测试实例。
func makeCapabilityPayloadTestCmd(t *testing.T, baseURL, payload string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "capability-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	if err := cmd.Flags().Set("token", "test-token"); err != nil {
		t.Fatalf("设置 token 测试参数失败: %v", err)
	}
	cmd.Flags().String("base-url", "", "")
	if err := cmd.Flags().Set("base-url", baseURL); err != nil {
		t.Fatalf("设置 base-url 测试参数失败: %v", err)
	}
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	if err := cmd.Flags().Set("payload", payload); err != nil {
		t.Fatalf("设置 payload 测试参数失败: %v", err)
	}
	return cmd
}
