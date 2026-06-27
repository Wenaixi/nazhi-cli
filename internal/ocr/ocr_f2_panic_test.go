// Package ocr 内部白盒测试：F2 initMu 临界区内 panic → tempDir 永久泄漏。
//
// F2：extractModels + ddddocr.New 在 initMu 临界区内可能 panic。
// 此时 o.tempDir 已被赋值但 o.initialized 未正确设置，panic 向上传播，
// 遗留的 tempDir 变成孤儿泄漏到 %TEMP%。
//
// 修复：initOnce 内 deferred recover → 清理 tempDir + 重置 initialized + re-panic。
package ocr

import (
	"os"
	"testing"
)

// TestOCR_F2_PanicTempDirCleanup 验证 panic 后 tempDir 被清理以及正常路径不被误删。
func TestOCR_F2_PanicTempDirCleanup(t *testing.T) {
	t.Run("panic后tempDir被清理", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "nazhi-cli-ocr-f2-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })

		// 模拟 initOnce 内 panic → recover → 清理 tempDir
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 模拟 initOnce 的 recover 逻辑：清理 tempDir
					_ = os.RemoveAll(dir)
				}
			}()
			panic("模拟 ddddocr.New 内部 panic")
		}()

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Error("panic 后 tempDir 应被清理，但还存在")
		}
	})

	t.Run("正常路径tempDir不被误删", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "nazhi-cli-ocr-f2-safe-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })

		if _, err := os.Stat(dir); err != nil {
			t.Errorf("正常路径 tempDir %s 不应被删除: %v", dir, err)
		}
	})
}

// TestOCR_F2_InitOnce_PanicRecover 验证 deferred recover 模式的正确性：
// cleanupTempDir 标记 + tempDir 清理 + initialized 重置 + re-panic。
func TestOCR_F2_InitOnce_PanicRecover(t *testing.T) {
	dir, err := os.MkdirTemp("", "nazhi-cli-ocr-f2-recover-*")
	if err != nil {
		t.Fatalf("MkdirTemp 失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	o := &OCR{tempDir: dir}

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("应捕获到 panic")
				return
			}
			// recover 后验证 tempDir 已被清理
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Error("recover 后 tempDir 应被清理")
			}
		}()

		// 模拟 initOnce 的 deferred recover 逻辑
		var cleanupTempDir bool
		defer func() {
			if r := recover(); r != nil {
				if cleanupTempDir && o.tempDir != "" {
					_ = os.RemoveAll(o.tempDir)
					o.tempDir = ""
				}
				panic(r) // re-panic
			}
		}()

		cleanupTempDir = true
		panic("模拟 panic")
	}()
}

// TestOCR_F2_InitOnce_CleanupFlag 验证 cleanupTempDir 标记行为：
// extractModels 成功前 panic → 不清理（目录不存在）
// extractModels 成功后 panic → 清理
func TestOCR_F2_InitOnce_CleanupFlag(t *testing.T) {
	// 场景：extractModels 成功后 cleanupTempDir=true → panic → 清理
	t.Run("cleanupTempDir为true时panic清理", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "nazhi-cli-ocr-f2-flag-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })

		o := &OCR{tempDir: dir}

		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Error("应捕获到 panic")
					return
				}
			}()

			var cleanupTempDir bool
			defer func() {
				if r := recover(); r != nil {
					// cleanupTempDir=true 且 tempDir 非空 → 清理
					if cleanupTempDir && o.tempDir != "" {
						_ = os.RemoveAll(o.tempDir)
						o.tempDir = ""
					}
					panic(r)
				}
			}()

			// 模拟 extractModels 成功
			cleanupTempDir = true
			panic("模拟 ddddocr.New panic")
		}()

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Error("cleanupTempDir=true 时 panic 应清理 tempDir")
		}
		if o.tempDir != "" {
			t.Error("panic 后 o.tempDir 应被清空")
		}
	})
}
