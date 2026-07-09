package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// TestMain_NoDoubleErrorOutput 回归测试：main.go 收到 cobra 参数解析错误时，
// 用 printEnvelope(envelope.Error(400, msg)) 输出到 stdout（JSON envelope），
// 而不是 printError(execErr) 输出到 stderr。
// 验证：
//   - stdout 输出 JSON envelope（含 status=error, code=400）
//   - stderr 无 cobra 默认的 "Error: unknown flag" 前缀（SilenceErrors 生效）
//   - 退出码通过 pendingExitCode 标记为 3（参数错误）
func TestMain_NoDoubleErrorOutput(t *testing.T) {
	// 暂存全局状态以恢复
	origStdout := os.Stdout
	origStderr := os.Stderr
	rootCmd.SetArgs([]string{"login", "--badflag"})
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		rootCmd.SetArgs(nil)
	}()

	// 捕获 stdout（printEnvelope 写到此处）
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stdout = stdoutW

	// 捕获 stderr（验证 cobra 无输出）
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stderr 失败: %v", err)
	}
	os.Stderr = stderrW

	// 模拟 main.go：Execute → 若 execErr != nil 则 printEnvelope(code=400)
	execErr := rootCmd.Execute()
	if execErr == nil {
		t.Fatal("Execute 应返回错误（传入了未知 flag --badflag）")
	}
	printEnvelope(envelope.Error(400, execErr.Error()))

	// 关 writer 让 reader 能读到 EOF
	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf bytes.Buffer
	if _, err := io.Copy(&stdoutBuf, stdoutR); err != nil {
		t.Fatalf("读取 stdout 失败: %v", err)
	}
	var stderrBuf bytes.Buffer
	if _, err := io.Copy(&stderrBuf, stderrR); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	stdoutOutput := stdoutBuf.String()
	stderrOutput := stderrBuf.String()

	// 关键断言 1："unknown flag" 字样出现在 stdout（printEnvelope JSON 中），
	// stderr 应该没有 cobra 默认的 "Error: unknown flag" 前缀
	if strings.Contains(stderrOutput, "Error: unknown flag") {
		t.Errorf("stderr 出现 cobra 默认 'Error: unknown flag' 前缀 → SilenceErrors 未生效，stderr: %q", stderrOutput)
	}
	// stderr 应为空（SilenceErrors + SilenceUsage）
	if len(stderrOutput) > 0 {
		t.Errorf("stderr 应为空，实际: %q", stderrOutput)
	}

	// 关键断言 2：stdout 必须包含 JSON envelope 的 error 字段
	if !strings.Contains(stdoutOutput, `"status": "error"`) {
		t.Errorf("stdout 应包含 JSON envelope status=error，实际: %q", stdoutOutput)
	}

	// 关键断言 3：code 应为 400（参数错误 → exit code 3）
	if !strings.Contains(stdoutOutput, `"code": 400`) {
		t.Errorf("stdout 应包含 code=400，实际: %q", stdoutOutput)
	}

	// 关键断言 4：pendingExitCode 被标记为 3
	if code := pendingExitCode.Load(); code != 3 {
		t.Errorf("pendingExitCode 应为 3，实际 %d", code)
	}
}

// TestRootCmd_HasSilenceFlags 直接断言 package-level rootCmd 已经设置了
// SilenceErrors 和 SilenceUsage（main.go init() 阶段生效）。
// 防止 init() 里忘了加导致回归。
func TestRootCmd_HasSilenceFlags(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors 应为 true（防止 cobra 自带重复错误输出）")
	}
	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage 应为 true（防止 cobra 自带重复 usage 输出）")
	}
}

// 引入 cobra 引用以让编译器保留 cobra 包导入（虽然 cobra 已被 rootCmd 间接引用）
var _ = cobra.NoArgs
