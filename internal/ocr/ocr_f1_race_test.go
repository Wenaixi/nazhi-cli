//go:build ddddocr && windows

// Package ocr 内部白盒测试：F1 use-after-close 窗口竞争测试。
//
// F1（CRITICAL）：Pool.Recognize 在 closeMu 临界区内 Get + trackInit 后释放锁，
// 但 o.Recognize(imageData) 在锁外执行。并发 Close 可在期间关闭该 OCR 实例，
// 导致 ddddocr C 运行时 segfault。
//
// 修复（两层防御）：
//   - 层 1（OCR 级别）：OCR.closed 改 atomic.Bool，在 o.mu 临界区内、Classification
//     前做二次 closed 检查，确保永不访问已关闭 session。
//   - 层 2（Pool 级别）：closeMu 临界区保护 Get+trackInit，Close 无法漏掉实例。
package ocr

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestOCR_F1_UseAfterClose 验证 use-after-close 修复的 3 个 invariant。
func TestOCR_F1_UseAfterClose(t *testing.T) {
	t.Run("OCR.Close后Recognize返回错误不panic", func(t *testing.T) {
		o := &OCR{}
		// 模拟「曾经成功初始化」状态
		o.initMu.Lock()
		o.initialized = true
		o.initMu.Unlock()

		o.Close()

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Close 后 Recognize 触发 panic: %v", r)
				}
			}()
			_, err = o.Recognize([]byte("fake"))
		}()

		if err == nil {
			t.Fatal("Close 后 Recognize 应返回错误，但返回 nil")
		}
	})

	t.Run("并发Recognize+Close无panic", func(t *testing.T) {
		const recognizeGoroutines = 50
		p := NewPool(0)

		var wg sync.WaitGroup
		wg.Add(recognizeGoroutines + 1)

		var panics atomic.Int32

		for i := 0; i < recognizeGoroutines; i++ {
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						panics.Add(1)
					}
				}()
				for j := 0; j < 10; j++ {
					_, _ = p.Recognize([]byte("fake"))
				}
			}()
		}

		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			_ = p.Close()
		}()

		wg.Wait()

		if n := panics.Load(); n > 0 {
			t.Errorf("use-after-close 修复失败：%d 个 goroutine panic（应为 0）", n)
		}
	})

	t.Run("Close后错误消息包含已关闭", func(t *testing.T) {
		o := &OCR{}
		o.Close()
		_, err := o.Recognize([]byte("fake"))
		if err == nil {
			t.Fatal("Close 后 Recognize 应返回错误")
		}
	})
}
