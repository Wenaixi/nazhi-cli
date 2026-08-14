package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHonorOptionCommandsRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, command := range honorCmd.Commands() {
		registered[command.Name()] = true
	}
	if !registered["type-options"] || !registered["level-options"] {
		t.Fatalf("honor 应注册 type-options 和 level-options，实际=%v", registered)
	}
	if honorTypeOptionsCmd.Flags().Lookup("token") == nil || honorTypeOptionsCmd.Flags().Lookup("base-url") == nil {
		t.Fatal("honor type-options 应注册业务通用参数")
	}
	if honorLevelOptionsCmd.Flags().Lookup("token") == nil || honorLevelOptionsCmd.Flags().Lookup("base-url") == nil {
		t.Fatal("honor level-options 应注册业务通用参数")
	}
}

func TestHonorTypeOptionsCmd_UsesDataList(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/studentMoralEduNew/getHonorTypeForSelect" {
			gotPath = r.URL.Path
			_, _ = w.Write([]byte(
				`{"code":1,"dataList":[{"label":"校三好学生","value":1147}],"returnData":[{"label":"校","value":5}]}`,
			))
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/api/studentInfo/getMenu" {
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		if r.URL.Path == "/api/studentInfo/getMyInfo" {
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cmd := makeHonorOptionsTestCmd(t, srv.URL, "test-token")
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})
	stdout, stderr, restore := captureStdio(t)
	honorTypeOptionsCmd.Run(cmd, nil)
	restore()

	if gotPath != "/api/studentMoralEduNew/getHonorTypeForSelect" {
		t.Fatalf("请求路径错误: %q", gotPath)
	}
	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功查询不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "校三好学生") || strings.Contains(stdout.String(), "通用") {
		t.Fatalf("类型选项未读取 dataList: %s", stdout.String())
	}
}

func TestHonorLevelOptionsCmd_UsesReturnData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/studentMoralEduNew/getHonorTypeForSelect" {
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"label":"校三好学生","value":1147}],"returnData":[{"label":"校","value":5}]}`))
			return
		}
		if r.URL.Path == "/" || r.URL.Path == "/api/studentInfo/getMenu" {
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
			return
		}
		if r.URL.Path == "/api/studentInfo/getMyInfo" {
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cmd := makeHonorOptionsTestCmd(t, srv.URL, "test-token")
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})
	stdout, stderr, restore := captureStdio(t)
	honorLevelOptionsCmd.Run(cmd, nil)
	restore()

	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功查询不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "校") || !strings.Contains(stdout.String(), "5") {
		t.Fatalf("等级选项未读取 returnData: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "校三好学生") {
		t.Fatalf("等级选项不应混入 dataList: %s", stdout.String())
	}
}

func TestHonorTypeOptionsCmd_RejectsMissingToken(t *testing.T) {
	cmd := makeHonorOptionsTestCmd(t, "http://127.0.0.1:1", "")
	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
	})
	stdout, stderr, restore := captureStdio(t)
	honorTypeOptionsCmd.Run(cmd, nil)
	restore()
	if !strings.Contains(stderr.String(), "--token") {
		t.Fatalf("缺少 token 应输出参数错误到 stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("参数错误不应写入 stdout: %s", stdout.String())
	}
	if pendingExitCode.Load() != 3 {
		t.Fatalf("缺少 token 应设置退出码 3，实际 %d", pendingExitCode.Load())
	}
}

func makeHonorOptionsTestCmd(t *testing.T, baseURL, token string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "honor-options-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", token)
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}
