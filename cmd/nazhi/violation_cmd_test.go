package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestViolationCommandsRegistered 验证 SDK 违规查询能力保留为 CLI 命令。
func TestViolationCommandsRegistered(t *testing.T) {
	var parentFound bool
	for _, command := range rootCmd.Commands() {
		if command.Name() != "violation" {
			continue
		}
		parentFound = true
		if len(command.Commands()) != 2 {
			t.Fatalf("violation 应包含 list/types 两个子命令，实际 %d", len(command.Commands()))
		}
	}
	if !parentFound {
		t.Fatal("根命令应注册 violation")
	}
}

// TestViolationListFlagsMatchFrontend 验证违规列表 CLI 暴露前端查询字段和默认值。
func TestViolationListFlagsMatchFrontend(t *testing.T) {
	for _, name := range []string{"page", "page-size", "key", "token", "base-url", "timeout"} {
		if violationListCmd.Flags().Lookup(name) == nil {
			t.Errorf("violation list 缺少 --%s", name)
		}
	}
	if flag := violationListCmd.Flags().Lookup("page"); flag == nil || flag.DefValue != "1" {
		t.Fatalf("违规列表默认 page 应为 1，实际 %#v", flag)
	}
	if flag := violationListCmd.Flags().Lookup("page-size"); flag == nil || flag.DefValue != "10" {
		t.Fatalf("违规列表默认 page-size 应为 10，实际 %#v", flag)
	}
}

// TestViolationListCommandPreservesFrontendQueryAndEnvelope 验证列表命令完整走 CLI 到 SDK。
func TestViolationListCommandPreservesFrontendQueryAndEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"示例学生","studentNumber":"TEST2025001"}}`))
		case "/api/studentMoralEduNew/getViolation":
			if r.Method != http.MethodGet {
				t.Errorf("违规列表必须使用 GET，实际 %s", r.Method)
			}
			if r.URL.Query().Get("key") != "迟到 记录" {
				t.Errorf("--key 未透传，实际 %q", r.URL.Query().Get("key"))
			}
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"name":"上课迟到","rawExtra":"keep"}],"pageBean":{"pageNo":2,"pageSize":10,"totalNum":1,"totalPage":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cmd := &cobra.Command{Use: "violation-list-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", server.URL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Int("page", 1, "")
	_ = cmd.Flags().Set("page", "2")
	cmd.Flags().Int("page-size", 10, "")
	cmd.Flags().String("key", "", "")
	_ = cmd.Flags().Set("key", "迟到 记录")

	originalStdout, originalStderr := os.Stdout, os.Stderr
	originalQuiet := quiet
	stdoutReader, stdoutWriter, _ := os.Pipe()
	stderrReader, stderrWriter, _ := os.Pipe()
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	quiet = false
	pendingExitCode.Store(0)
	defer func() {
		os.Stdout, os.Stderr = originalStdout, originalStderr
		quiet = originalQuiet
		_ = closeAllClients()
	}()

	violationListCmd.Run(cmd, nil)
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	var stdout, stderr bytes.Buffer
	_, _ = io.Copy(&stdout, stdoutReader)
	_, _ = io.Copy(&stderr, stderrReader)

	if pendingExitCode.Load() != 0 {
		t.Fatalf("成功列表命令不应设置退出码，实际 %d，stderr=%s", pendingExitCode.Load(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "success"`) || !strings.Contains(stdout.String(), `"rawExtra": "keep"`) {
		t.Fatalf("CLI 输出应为成功 envelope 且保留原始字段，实际: %s", stdout.String())
	}
}
