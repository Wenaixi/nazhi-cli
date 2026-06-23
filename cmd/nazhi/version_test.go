package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/Wenaixi/nazhi-cli/internal/version"
)

// TestVersionCommand 验证 `nazhi version` 输出 JSON 格式的版本号。
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

	// 验证 JSON 格式输出
	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("输出应为 JSON 格式，解析失败: %v (原始输出: %q)", err, output)
	}

	v, ok := result["version"]
	if !ok {
		t.Fatalf("JSON 输出缺少 version 字段: %q", output)
	}
	if v != version.Version {
		t.Errorf("version 字段应为 %q，实际: %q", version.Version, v)
	}
}
