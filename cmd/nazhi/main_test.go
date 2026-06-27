package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestMain_NoDoubleErrorOutput 回归测试：修复后引入的副作用——
// cobra 在遇到 parse error 时默认往 stderr 打印 "Error: ..." 和 Usage。
// 同时 main.go:42 又用 fmt.Fprintln 再打一次同一条 error
// 结果用户看到两遍错误信息（第二遍还没 "Error:" 前缀，破坏 JSON 统一契约）。
// 修复方案
//  1. rootCmd.SilenceErrors = true   → 让 cobra 不再自带 "Error: ..." 打印
//  2. rootCmd.SilenceUsage  = true   → 让 cobra 不再自带 "Usage:" 打印
//  3. main.go:42 把 fmt.Fprintln 换成 printError(execErr)
//     → 走和 Run 回调相同的 JSON envelope 路径
//
// 本测试模拟 main.go 的执行流程：用真实的 package-level rootCmd
// 传入未知 flag，捕获 stderr，验证
//   - "unknown flag" 字样只出现 1 次（不是 2 次）
//   - 不出现 cobra 默认的 "Error: unknown flag" 前缀
//   - 出现 JSON envelope `\`"error\`: true`（printError 路径生效）
//
// 注意：本测试在测试结束前会恢复 rootCmd 的全局标志和 SetArgs 副作用
// 避免污染其它测试。
func TestMain_NoDoubleErrorOutput(t *testing.T) {
	// 暂存全局状态以恢复
	origStderr := os.Stderr
	rootCmd.SetArgs([]string{"login", "--badflag"})
	defer func() {
		os.Stderr = origStderr
		rootCmd.SetArgs(nil)
	}()

	// 捕获 stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w

	// 模拟 main.go：Execute → 若 execErr != nil 则 printError(execErr)
	execErr := rootCmd.Execute()
	if execErr == nil {
		t.Fatal("Execute 应返回错误（传入了未知 flag --badflag）")
	}
	printError(execErr)

	// 关 writer 让 reader 能读到 EOF
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	stderrOutput := buf.String()

	// 关键断言 1：bug 修复的标志——"unknown flag" 字样只能出现 1 次
	count := strings.Count(stderrOutput, "unknown flag")
	if count != 1 {
		t.Errorf("F2 未修复: stderr 中 'unknown flag' 出现 %d 次（应为 1 次），完整输出:\n%s", count, stderrOutput)
	}

	// 关键断言 2：单源输出应通过 printError 的 JSON envelope
	// 不能出现 cobra 默认的 "Error: unknown flag" 前缀。
	// 说明 SilenceErrors 没生效。
	if strings.Contains(stderrOutput, "Error: unknown flag") {
		t.Errorf("stderr 出现 cobra 默认 'Error: unknown flag' 前缀 → SilenceErrors 未生效，完整输出:\n%s", stderrOutput)
	}

	// 关键断言 3：必须包含 JSON envelope 的 error 字段
	if !strings.Contains(stderrOutput, `"error": true`) {
		t.Errorf("stderr 应包含 JSON envelope `\\\"error\\\": true`，完整输出:\n%s", stderrOutput)
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
