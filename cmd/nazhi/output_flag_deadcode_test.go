package main

import (
	"testing"
)

// TestOutputFlag_IsNotDeadCode F2 GREEN 测试：验证 --output flag 已被删除。
// 原 --output flag 注册了但全仓 0 reader，属于死代码。
// 修复后 flag 不再存在，所有相关测试应 assert 其不存在。
func TestOutputFlag_IsNotDeadCode(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("output")
	if f != nil {
		t.Error("F2 未修复: --output flag 仍存在但全仓 0 reader（死代码）")
	}
}
