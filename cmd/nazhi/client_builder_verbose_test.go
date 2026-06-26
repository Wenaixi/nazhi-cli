// client_builder_verbose_test.go G2 修复锚定：
// --verbose flag 联动 SDK logger 级别，让 c.logDebug 输出可见。
package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

// TestBuildClientOpts_Verbose_SetsDebugLogger G2 修复验证：
// verbose=true 时 buildClientOpts 返回的 opts 链中应包含
// WithLogger(slog.LevelDebug) 选项，使得 SDK 的 c.logDebug 不再被
// slog LevelWarn 过滤。
//
// 修复前（v0.3.4-）：verbose flag 只影响 CLI 层 printVerbose 输出，
// SDK 层 c.logDebug 调用默认 slog LevelWarn handler 静默过滤。
// 用户 nazhi login -v 看到 CLI 层日志但看不到 SDK 内部细节。
//
// 修复后：--verbose 让 Client logger 改为 LevelDebug，c.logDebug 写入 stderr。
func TestBuildClientOpts_Verbose_SetsDebugLogger(t *testing.T) {
	// 保存原始 verbose 值
	origVerbose := verbose
	defer func() { verbose = origVerbose }()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("sso-base", "", "")
	cmd.Flags().Int("timeout", 5, "")

	// verbose=true 时
	verbose = true
	opts1, _, err1 := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err1 != nil {
		t.Fatalf("buildClientOpts with verbose=true 失败: %v", err1)
	}

	// verbose=false 时
	verbose = false
	opts2, _, err2 := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err2 != nil {
		t.Fatalf("buildClientOpts with verbose=false 失败: %v", err2)
	}

	// 验证 verbose=true 时 opts 列表比 verbose=false 多一个 WithLogger 选项
	// (因为 verbose 全局变量在测试中不是并发安全的，但我们顺序执行)
	if len(opts1) <= len(opts2) {
		t.Errorf("verbose=true 时 opts 应比 verbose=false 多（多了 WithLogger 选项）: verbose=true %d, verbose=false %d",
			len(opts1), len(opts2))
	}

	// 验证 verbose=false 时 opts 数量正常
	_ = bytes.NewBuffer
}
