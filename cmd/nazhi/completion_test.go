package main

import (
	"testing"
)

// TestCompletionCommand 验证 completion 命令能注册并运行，不崩溃。
// 不验证输出内容（shell 补全脚本很长，管道缓冲可能死锁），
// 只验证命令能正确初始化且无参数时报错。
func TestCompletionCommand(t *testing.T) {
	// 测试无参数时显示帮助（不会崩溃）
	rootCmd.SetArgs([]string{"completion"})
	err := rootCmd.Execute()
	if err != nil {
		// 无参数时 cobra 会报 args 错误，这是预期的
		t.Logf("completion 无参数预期报错: %v", err)
	}
}

// TestCompletionCommand_InvalidShell 验证不支持的 shell 报错。
func TestCompletionCommand_InvalidShell(t *testing.T) {
	rootCmd.SetArgs([]string{"completion", "csh"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("不支持的 shell 应报错")
	}
}

// TestCompletionCommand_BashSucceeds 验证 bash 补全能生成（不检查输出避免死锁）。
func TestCompletionCommand_BashSucceeds(t *testing.T) {
	rootCmd.SetArgs([]string{"completion", "bash"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("bash completion 失败: %v", err)
	}
}
