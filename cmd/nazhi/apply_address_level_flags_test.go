package main

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestApplyAddressLevelFlags 锁定 --address/--level 覆盖语义的真实现
// （write_op_runner.go 的 applyAddressLevelFlags）。此前的测试替身
// applyTaskWriteFlags（map 版）锁的是生产中不存在的实现——生产实际是
// 4 份逐字重复的匿名闭包（submit/edit/previewSubmit/previewEdit），
// 闭包内的 cmd.Flags().GetString 读取从未被测试触达。
//
// 本测试直调收敛后的真函数，锁定三件事：
//  1. 非空 flag 覆盖 payload 原值
//  2. 空 flag 保留 payload 原值
//  3. flag 未注册时静默空串（pflag GetString 报错被忽略）→ 保留 payload 原值
func TestApplyAddressLevelFlags(t *testing.T) {
	t.Run("非空 flag 覆盖 payload 值", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("address", "", "")
		cmd.Flags().String("level", "", "")
		_ = cmd.Flags().Set("address", "flag地址")
		_ = cmd.Flags().Set("level", "5")

		address, level := "payload地址", "3"
		applyAddressLevelFlags(cmd, func(a, l string) {
			address, level = a, l
		})
		if address != "flag地址" || level != "5" {
			t.Fatalf("非空 flag 应覆盖: got address=%q level=%q", address, level)
		}
	})

	t.Run("空 flag 传给 apply 空串（由闭包判空保留原值）", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("address", "", "")
		cmd.Flags().String("level", "", "")
		// 不 Set：flag 保持默认空串

		gotA, gotL := "sentinel", "sentinel"
		applyAddressLevelFlags(cmd, func(a, l string) {
			gotA, gotL = a, l
		})
		if gotA != "" || gotL != "" {
			t.Fatalf("空 flag 应传给 apply 空串: got address=%q level=%q", gotA, gotL)
		}
	})

	t.Run("flag 未注册时传给 apply 空串（GetString 错误被忽略）", func(t *testing.T) {
		cmd := &cobra.Command{}
		// 不注册 address/level：GetString 报错被忽略，取零值空串

		gotA, gotL := "sentinel", "sentinel"
		applyAddressLevelFlags(cmd, func(a, l string) {
			gotA, gotL = a, l
		})
		if gotA != "" || gotL != "" {
			t.Fatalf("flag 未注册应传空串: got address=%q level=%q", gotA, gotL)
		}
	})
}
