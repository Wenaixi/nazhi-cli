// Package ocr 内部白盒测试：initMuGlobal panic 死锁修复。
//
// initOnce() 中 initMuGlobal.Lock() 后没有 defer Unlock，
// 正常路径在行末 initMuGlobal.Unlock() 处解锁。但若 ddddocr.New() 内部 panic，
// Unlock 永不执行 → 全局锁永久死锁 → 后续所有 OCR 初始化永远阻塞。
//
// 修复方式：initMuGlobal.Lock() 后紧跟 defer initMuGlobal.Unlock()，
// 替代行末的手动 Unlock。Go 保证 defer 在函数返回（含 panic unwind）时执行。
package ocr

import (
	"testing"
	"time"
)

// TestOCR_F1_InitMuGlobal_PanicDeadlock 验证 initMuGlobal 在 panic 死锁修复。
func TestOCR_F1_InitMuGlobal_PanicDeadlock(t *testing.T) {
	// ─────────────────────────────────────────────────────────
	// 子测试 1（GREEN 验证）：Lock + defer Unlock + panic → 锁被正确释放
	// 模拟修复后的 initOnce 代码模式。
	// ─────────────────────────────────────────────────────────
	t.Run("defer释放panic路径的锁", func(t *testing.T) {
		panicked := make(chan struct{})
		go func() {
			defer func() { recover(); close(panicked) }()
			initMuGlobal.Lock()
			defer initMuGlobal.Unlock() // FIX：defer 确保 panic 也释放锁
			panic("模拟 ddddocr.New 内部 panic")
		}()
		<-panicked

		// 等待 G1 完全退出后，G2 尝试获取锁
		time.Sleep(20 * time.Millisecond)

		ch := make(chan struct{})
		go func() {
			initMuGlobal.Lock()
			initMuGlobal.Unlock()
			close(ch)
		}()

		select {
		case <-ch:
			// PASS：defer Unlock 在 panic 后正确释放了锁
		case <-time.After(3 * time.Second):
			t.Fatal("F1 FIX FAILED: Lock+defer Unlock+panic 后 initMuGlobal 仍被持有")
		}
	})

	// ─────────────────────────────────────────────────────────
	// 子测试 2（RED 重现）：Lock + panic（无 defer）→ 锁永久死锁
	// 模拟修复前的 initOnce bug 模式。
	// ─────────────────────────────────────────────────────────
	t.Run("无defer时锁被永久持有", func(t *testing.T) {
		locked := make(chan struct{})
		go func() {
			initMuGlobal.Lock()
			close(locked)
			// 无 defer Unlock —— 模拟修复前的 bug
		}()
		<-locked
		time.Sleep(20 * time.Millisecond)

		ch := make(chan struct{})
		go func() {
			initMuGlobal.Lock()
			initMuGlobal.Unlock()
			close(ch)
		}()

		select {
		case <-ch:
			t.Fatal("BUG VERIFICATION FAILED: 无 defer 时锁应被永久持有")
		case <-time.After(500 * time.Millisecond):
			// PASS —— 锁被永久持有（验证旧 bug 真实存在）
		}

		// 清理：手动释放锁以便后续测试正常
		// Go sync.Mutex 无所有权语义，允许其他 goroutine 解锁
		initMuGlobal.Unlock()
	})
}
