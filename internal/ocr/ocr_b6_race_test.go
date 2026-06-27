// Package ocr 内部白盒测试：SetOnnxRuntimePath 全局状态竞争测试。
package ocr

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// TestRecognize_ConcurrentInitNoRace 验证多个 OCR 实例并发初始化不触发
// SetOnnxRuntimePath 和 ddddocr.New 的全局状态竞争。
func TestRecognize_ConcurrentInitNoRace(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("竞争测试只需要在 Windows 上验证初始化路径的稳定性，其他平台跳过")
	}

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			p := NewPool(0)
			defer p.Close()
			_, _ = p.Recognize([]byte("fake-image-data"))
		}(i)
	}
	wg.Wait()
}

// TestRecognize_ConcurrentInitPanicFree 轻量级验证：多个 goroutine 并发
// 第一次调 Recognize 不应因全局状态竞争而 panic。
func TestRecognize_ConcurrentInitPanicFree(t *testing.T) {
	const goroutines = 4
	var (
		wg     sync.WaitGroup
		panics atomic.Int32
	)
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()

			p := NewPool(0)
			defer p.Close()
			_, _ = p.Recognize([]byte("fake"))
		}()
	}
	wg.Wait()

	if n := panics.Load(); n > 0 {
		t.Errorf("并发初始化时 %d 个 goroutine panic，有回归", n)
	}
}
