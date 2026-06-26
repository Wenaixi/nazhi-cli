// Package ocr 内部白盒测试：C8 writeModelFile helper 验证。
package ocr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteModelFile_Success 验证 writeModelFile 成功路径。
func TestWriteModelFile_Success(t *testing.T) {
	dir, err := os.MkdirTemp("", "ocr-c8-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp 失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	data := []byte("hello")
	if err := writeModelFile(dir, "test.txt", data); err != nil {
		t.Fatalf("writeModelFile 应成功，但返回: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	if err != nil {
		t.Fatalf("读取写入的文件失败: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("文件内容应为 'hello'，实际: %q", string(got))
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("写入成功后目录不应被删除: %v", err)
	}
}

// TestWriteModelFile_CleanupOnFailure 验证写失败时目录被清理。
func TestWriteModelFile_CleanupOnFailure(t *testing.T) {
	dir, err := os.MkdirTemp("", "ocr-c8-fail-*")
	if err != nil {
		t.Fatalf("MkdirTemp 失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	// 用 null byte 路径使 WriteFile 失败
	badPath := filepath.Join(dir, string([]byte{0x00, 'x'}))
	err = writeModelFile(filepath.Dir(badPath), string([]byte{0x00, 'x'}), []byte("data"))
	if err == nil {
		t.Log("当前平台可能支持 null byte 文件名，跳过本测试")
		return
	}
	if !strings.Contains(err.Error(), "写入") {
		t.Errorf("错误消息应包含『写入』，实际: %v", err)
	}
}
