package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHonorLevelsCommandRegistered(t *testing.T) {
	var found bool
	for _, command := range honorCmd.Commands() {
		if command.Name() == "levels" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("honor 应注册 levels 子命令")
	}
	if honorLevelsCmd.Flags().Lookup("type-id") == nil {
		t.Fatal("honor levels 应暴露 --type-id")
	}
}

func TestHonorLevelsCmd_SendsHonorTypeID(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/getHonorLevel":
			gotPath = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"label":"校","value":5}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cmd := &cobra.Command{Use: "honor-levels"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", srv.URL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Int64("type-id", 0, "")
	_ = cmd.Flags().Set("type-id", "1147")

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
	honorLevelsCmd.Run(cmd, nil)
	restore()

	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功查询不应设置退出码，实际 %d；stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if !strings.Contains(gotPath, "honorTypeId=1147") {
		t.Fatalf("应透传 honorTypeId=1147，实际 query=%s", gotPath)
	}
	out := stdout.String()
	if !strings.Contains(out, "校") || !strings.Contains(out, "5") {
		t.Fatalf("级别下拉未透传到 envelope: %s", out)
	}
}
