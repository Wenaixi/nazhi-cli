package main

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
)

func TestParsePayloadFromArgRejectsOversizedStdin(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "payload-*.json")
	if err != nil {
		t.Fatalf("创建临时 stdin 文件失败: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(strings.Repeat("x", 16<<20+1)); err != nil {
		t.Fatalf("写入超限 stdin 失败: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("重置 stdin 文件位置失败: %v", err)
	}

	originalStdin := os.Stdin
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = originalStdin })

	_, err = parsePayloadFromArg(context.Background(), "-")
	if err == nil {
		t.Fatal("超过 16 MiB 的 stdin payload 应返回错误")
	}
	if !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("超限错误应说明大小限制，实际: %v", err)
	}
}

// TestParsePayloadFromArg_StdinPipelineStillWorks 锁定管道场景（预期用法）：
// 接入 printPrompt/readStdinWithTimeout 保护后，非终端 stdin（CI/管道）必须
// 照常读到内容——提示符受 isTerminalStdin 守卫不应污染，60s 超时不应误触发。
func TestParsePayloadFromArg_StdinPipelineStillWorks(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "payload-*.json")
	if err != nil {
		t.Fatalf("创建临时 stdin 文件失败: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(`{"taskId":1}`); err != nil {
		t.Fatalf("写入 stdin 失败: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("重置 stdin 文件位置失败: %v", err)
	}

	originalStdin := os.Stdin
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = originalStdin })

	got, err := parsePayloadFromArg(context.Background(), "-")
	if err != nil {
		t.Fatalf("管道 stdin 应正常读取，实际错误: %v", err)
	}
	if !strings.Contains(string(got), "taskId") {
		t.Fatalf("stdin 内容应原样返回，实际: %s", string(got))
	}
}

// TestPayloadPositiveIDValid_MathTrunc（C3 修正版）锁定：float64 整数判定改用
// math.Trunc 对齐 FlexInt（）。旧实现 `v == float64(int64(v))` 对
// 2^53..2^63 区间的整数字面量（float64 无法精确表示相邻整数，int64 转换回绕）
// 会静默误判——如 float64(9007199254740993) 舍入为 9007199254740992 与
// int64 转换相同值，v 恒等于回绕值；改用 math.Trunc 判整数 + 上界拒绝。
func TestPayloadPositiveIDValid_MathTrunc(t *testing.T) {
	cases := []struct {
		id       float64
		name     string
		expected bool
	}{
		{1, "普通正数", true},
		{2.0, "整数", true},
		{0, "零", false},
		{-5, "负数", false},
		{1.5, "小数", false},
		// 2^53+1 字面量在 float64 解码层已舍入为 9007199254740992（偶数），
		// 仍是合法正整数——float64 精度损失是 json 解码的固有属性，不是本
		// 校验的职责（需保真应走 json.Number）。
		{9007199254740993, "2^53+1（float64 舍入后为合法整数）", true},
		{float64(math.MaxInt64), "2^63-1（float64 舍入为 2^63）", false},
		{1 << 63, "2^63 越界", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PayloadPositiveIDValid(map[string]any{"id": c.id})
			if got != c.expected {
				t.Fatalf("PayloadPositiveIDValid(%v) = %v, want %v", c.id, got, c.expected)
			}
		})
	}
}
