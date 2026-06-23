// Package ocr 内部白盒测试。
package ocr

import (
	"strings"
	"testing"
)

// TestOCR_CloseReturnsRemoveAllError 回归测试：Close() 必须返回 os.RemoveAll
// 的清理错误，不能静默吞掉（Windows AV 持锁 / Linux 权限拒绝 / 任意 platform
// 错误都应让调用方知情）。
//
// 历史 bug：Close() 无条件 return nil，导致 temp dir 永久泄漏到 %TEMP%。
func TestOCR_CloseReturnsRemoveAllError(t *testing.T) {
	// 用 null byte 路径让 os.RemoveAll 必失败（跨平台、零依赖）
	o := &OCR{tempDir: "\x00invalid-tempdir-for-test"}

	err := o.Close()
	if err == nil {
		t.Fatal("Close() 应返回 os.RemoveAll 错误，但返回 nil")
	}
	if !strings.Contains(err.Error(), "清理临时目录") {
		t.Errorf("错误信息应说明 '清理临时目录' 失败，实际：%v", err)
	}
}
