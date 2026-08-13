package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func makeSelfEvalGradStatusTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/querySelfGradEvaluation":
			_, _ = w.Write([]byte(`{"code":1,"dataMap":{"student_comment":"毕业感言","isGrad":1,"rawGradExtra":"keep_me"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	cmd := &cobra.Command{Use: "self-eval-grad-status"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

func TestSelfEvalGradStatusCommandRegistered(t *testing.T) {
	var found bool
	for _, command := range selfEvalCmd.Commands() {
		if command.Name() == "grad-status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("self-eval 应注册 grad-status 子命令")
	}
}

func TestSelfEvalGradStatusCmd_PreservesFrontendDataMap(t *testing.T) {
	cmd := makeSelfEvalGradStatusTestCmd(t)
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
	selfEvalGradStatusCmd.Run(cmd, nil)
	restore()

	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功查询不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "keep_me") || !strings.Contains(out, "student_comment") {
		t.Fatalf("毕业评价原始字段未透传: %s", out)
	}
}

func TestSelfEvalGradSubmitCmd_SendsStudentComment(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/addSelfGradEvaluation":
			if r.Method != http.MethodPost {
				t.Errorf("毕业评价提交必须使用 POST，实际 %s", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cmd := &cobra.Command{Use: "self-eval-grad-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("comment", "", "")
	_ = cmd.Flags().Set("comment", "毕业感言……")

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
	selfEvalGradSubmitCmd.Run(cmd, nil)
	restore()

	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功提交不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if gotBody["studentComment"] != "毕业感言……" {
		t.Fatalf("毕业评价请求体不是单层 studentComment: %#v", gotBody)
	}
	if !strings.Contains(stdout.String(), `"code": 204`) {
		t.Fatalf("毕业评价提交应输出成功 envelope，实际: %s", stdout.String())
	}
}
