package main

import (
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

	_, err = parsePayloadFromArg("-")
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

	got, err := parsePayloadFromArg("-")
	if err != nil {
		t.Fatalf("管道 stdin 应正常读取，实际错误: %v", err)
	}
	if !strings.Contains(string(got), "taskId") {
		t.Fatalf("stdin 内容应原样返回，实际: %s", string(got))
	}
}
