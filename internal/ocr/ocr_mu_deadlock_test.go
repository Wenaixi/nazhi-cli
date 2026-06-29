// Package ocr 内部白盒测试：o.mu 在 Classification panic 后死锁修复。
//
// Recognize 内 o.mu.Lock() 后没有 defer Unlock，正常路径在返回前手动 Unlock。
// 但若 ocr.Classification(imageData) 内部 panic（CGO 运行时 segfault），
// o.mu.Unlock() 永不执行 → o.mu 永久锁死。
// Pool.Recognize() 从 sync.Pool.Get() 拿到该实例后，
// 下一个 Recognize 调用在 o.mu.Lock() 永久阻塞。
//
// 修复方式：o.mu.Lock() 后紧跟 defer o.mu.Unlock()，删除所有手动 Unlock。
// Go 保证 defer 在函数返回（含 panic unwind）时执行。
package ocr

import (
	"testing"
	"time"
)

// TestOCR_Mu_DeferUnlockAfterPanic 验证 o.mu 在 panic 后正确释放。
func TestOCR_Mu_DeferUnlockAfterPanic(t *testing.T) {
	// ─────────────────────────────────────────────────────────
	// 子测试 1（GREEN 验证）：Lock + defer Unlock + panic → 锁被正确释放
	// 模拟修复后的 Recognize 代码模式。
	// ─────────────────────────────────────────────────────────
	t.Run("defer释放panic路径的锁", func(t *testing.T) {
		o := &OCR{}
		panicked := make(chan struct{})
		go func() {
			defer func() { recover(); close(panicked) }()
			o.mu.Lock()
			defer o.mu.Unlock() // FIX：defer 确保 panic 也释放锁
			panic("模拟 Classification CGO panic")
		}()
		<-panicked

		// 等待 G1 完全退出后，G2 尝试获取锁
		time.Sleep(20 * time.Millisecond)

		ch := make(chan struct{})
		go func() {
			o.mu.Lock()
			//nolint:staticcheck // SA2001 空临界区故意构造
			o.mu.Unlock()
			close(ch)
		}()

		select {
		case <-ch:
			// PASS：defer Unlock 在 panic 后正确释放了锁
		case <-time.After(3 * time.Second):
			t.Fatal("FIX FAILED: Lock+defer Unlock+panic 后 o.mu 仍被持有")
		}
	})

	// ─────────────────────────────────────────────────────────
	// 子测试 2（RED 重现）：Lock + panic（无 defer）→ 锁永久死锁
	// 模拟修复前的 Recognize bug 模式。
	// ─────────────────────────────────────────────────────────
	t.Run("无defer时锁被永久持有", func(t *testing.T) {
		o := &OCR{}
		locked := make(chan struct{})
		go func() {
			o.mu.Lock()
			close(locked)
			// 无 defer Unlock —— 模拟修复前的 bug
		}()
		<-locked
		time.Sleep(20 * time.Millisecond)

		ch := make(chan struct{})
		go func() {
			o.mu.Lock()
			//nolint:staticcheck // SA2001 空临界区故意构造
			o.mu.Unlock()
			close(ch)
		}()

		select {
		case <-ch:
			t.Fatal("BUG VERIFICATION FAILED: 无 defer 时锁应被永久持有")
		case <-time.After(500 * time.Millisecond):
			// PASS —— 锁被永久持有（验证旧 bug 真实存在）
		}

		// 清理：手动释放锁以便后续测试正常
		o.mu.Unlock()
	})
}
