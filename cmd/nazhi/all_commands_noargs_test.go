package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestAllLeafCommands_NoArgs 锁定 CLI 一致性契约（P2-3 修复回归）：
// 全部叶子命令（无子命令者）必须声明 Args 校验，位置参数不得被 cobra 默认
// ArbitraryArgs 静默吞掉。completion 是唯一有意接受位置参数的叶子（ExactArgs(1)）。
func TestAllLeafCommands_NoArgs(t *testing.T) {
	var leaves []*cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if len(c.Commands()) == 0 {
			leaves = append(leaves, c)
			return
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)

	if len(leaves) == 0 {
		t.Fatal("未遍历到任何叶子命令（rootCmd 为空？）")
	}
	for _, c := range leaves {
		// completion 用 cobra.MatchAll(ExactArgs(1), OnlyValidArgs) 有意接受 shell 参数
		if c.Use == "completion [bash|zsh|fish|powershell]" {
			continue
		}
		if c.Args == nil {
			t.Errorf("叶子命令 %q 缺少 Args 声明（cobra 默认静默吞位置参数），Use=%q", c.Name(), c.Use)
			continue
		}
		if err := c.Args(c, []string{"多余"}); err == nil {
			t.Errorf("叶子命令 %q 应拒绝位置参数，实际放行", c.Name())
		}
	}
}
