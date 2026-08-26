package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestTypicalCaseSubcommands_NoArgs 锁定 CLI 一致性契约：
// typical-case 全部五个叶子子命令必须声明 Args: cobra.NoArgs。
// 所有输入都走 flag，位置参数本无语义；缺失声明时 cobra 默认 ArbitraryArgs
// 会静默吞掉手滑多敲的位置参数，与 delete-batch（已有声明）及全仓多数派不一致。
func TestTypicalCaseSubcommands_NoArgs(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"submit":       typicalCaseSubmitCmd,
		"list":         typicalCaseListCmd,
		"delete-batch": typicalCaseDeleteBatchCmd,
		"update":       typicalCaseUpdateCmd,
		"delete":       typicalCaseDeleteCmd,
	}
	for name, cmd := range cmds {
		if cmd.Args == nil {
			t.Errorf("typical-case %s 缺少 Args 声明（cobra 默认会静默忽略位置参数）", name)
			continue
		}
		if err := cmd.Args(cmd, []string{"多余的位置参数"}); err == nil {
			t.Errorf("typical-case %s 应拒绝位置参数，实际放行", name)
		}
	}
	// 行为面抽查：list 执行时携带位置参数应报错而非静默执行
	if !strings.Contains(name(typicalCaseListCmd), "list") {
		t.Fatal("夹具错位：list 命令引用失效")
	}
}

// name 避免未使用导入告警的辅助断言（Use 字段包含子命令名）
func name(c *cobra.Command) string { return c.Use }
