package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/Wenaixi/nazhi-cli/internal/version"
)

// TestVersionCommand 验证 `nazhi version` 输出版本号。
func TestVersionCommand(t *testing.T) {
	// 捕获 stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"version"})
	err := rootCmd.Execute()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("nazhi version 执行失败: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("版本输出为空")
	}
	expected := version.Version
	if output[:len(expected)] != expected {
		t.Errorf("版本输出应以 %q 开头，实际: %q", expected, output[:min(len(expected), len(output))])
	}
}
