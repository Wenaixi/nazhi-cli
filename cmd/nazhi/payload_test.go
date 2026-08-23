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
