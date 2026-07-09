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

	// 验证 JSON 格式输出（envelope 是 map[string]any，version 是 string）
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("输出应为 JSON 格式，解析失败: %v (原始输出: %q)", err, output)
	}

	// envelope 包了一层 data → version
	data, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatalf("JSON 缺少 envelope.data: %q", output)
	}
	v, ok := data["version"].(string)
	if !ok {
		t.Fatalf("data.version 不是 string: %q", output)
	}
	if v != version.Version {
		t.Errorf("version 字段应为 %q，实际: %q", version.Version, v)
	}
}
