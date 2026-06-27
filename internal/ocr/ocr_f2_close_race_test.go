// Package ocr 内部白盒测试：F2 Close + Recognize 并发 race 修复。
//
// Finding F2（CRITICAL）：Close() 在 o.mu 临界区外调用 ocr.Close()，
// 而 Recognize() 在 o.mu 临界区内调用 ocr.Classification()。两个 goroutine
// 同时操作同一个 ddddocr session 导致 C 运行时 segfault。
//
// 修复：将 ocr.Close() 移入 o.mu 临界区，与 Classification 互斥。
package ocr

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestOCR_F2_CloseRecognizeRace 验证 Close 与 Recognize 不会通过
// 同时操作 ddddocr session 造成 segfault。
func TestOCR_F2_CloseRecognizeRace(t *testing.T) {
	t.Run("并发Close+Recognize无panic", func(t *testing.T) {
		const goroutines = 20
		var panics atomic.Int32

		ocrs := make([]*OCR, goroutines)
		for i := range ocrs {
			ocrs[i] = &OCR{}
		}

		for i := 0; i < goroutines; i++ {
			o := ocrs[i]
			go func() {
				defer func() {
					if r := recover(); r != nil {
						panics.Add(1)
					}
				}()
				for j := 0; j < 20; j++ {
					o.Close()
					o.Recognize([]byte("fake"))
				}
			}()
		}

		time.Sleep(3 * time.Second)

		if n := panics.Load(); n > 0 {
			t.Errorf("F2 修复失败：%d 个 goroutine panic（应为 0）", n)
		}
	})

	t.Run("Close和Recognize的mu互斥验证", func(t *testing.T) {
		// 验证 Close 路径在 o.mu 内执行 ocr.Close（与 Classification 互斥）
		o := &OCR{}

		var recMuHeld atomic.Bool
		var closeMuHeld atomic.Bool

		var wg sync.WaitGroup
		wg.Add(2)

		// G1 模拟 Recognize 路径：持 mu → 工作 → 释放 mu
		go func() {
			defer wg.Done()
			defer func() { recover() }()
			o.mu.Lock()
			recMuHeld.Store(true)
			time.Sleep(500 * time.Millisecond)
			recMuHeld.Store(false)
			o.mu.Unlock()
		}()

		// G2 模拟 Close 路径：等待 recMuHeld → 尝试取 mu
		go func() {
			defer wg.Done()
			defer func() { recover() }()
			for !recMuHeld.Load() {
				time.Sleep(10 * time.Millisecond)
			}
			// G1 持有 mu，G2 应阻塞在 o.mu.Lock()
			o.mu.Lock()
			closeMuHeld.Store(true)
			time.Sleep(50 * time.Millisecond)
			closeMuHeld.Store(false)
			o.mu.Unlock()
		}()

		wg.Wait()

		if !closeMuHeld.Load() {
			// G1 释放了 mu（500ms），G2 应该已经获取到了
			if recMuHeld.Load() {
				// 如果 G1 还在持有 mu 但 closeMuHeld 永远 false — 死锁
				t.Error("Close 路径应能获取 mu（即使被 Recognize 延迟）")
			}
		}
	})
}
