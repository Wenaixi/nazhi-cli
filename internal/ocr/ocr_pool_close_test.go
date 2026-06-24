// Package ocr 内部白盒测试：Pool.Close 并发安全。
package ocr

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

// TestPool_Close_ConcurrentIsIdempotent 回归测试 F1：Pool.Close() 必须并发安全。
//
// 历史问题：原实现分两段（先拿 initsMu 排空 map，再迭代 Close 实例）。
// 两个 goroutine 同时 Close 时，第一个排空 map 后开始 Close 实例，
// 第二个也排空（已经空了）→ 立即返回 nil，看似无害；
// 但**如果第二个 goroutine 在第一个"拿到 inits 但还没替换 p.inits"之间挤进来**，
// 两者都会拿到**同一份 inits**，于是同一批实例被 Close 两遍
// （OCr.Close 内部 ocr.Close + tempDir RemoveAll 重复执行）。
//
// 修复方案：Pool 加 sync.Once，第一次 Close 负责排空 + 迭代；后续 no-op。
//
// 测试策略：直接给 Pool 注入 N 个独立 OCR 实例（绕过 NewPool 的 sync.Pool 复用
// 行为——sync.Pool.Get/Put 同一对象的副作用与本测试无关），每个实例装上 closeHook
// 计数 + 真实 tempDir（避免 OCR.Close 失败干扰断言）。
// 然后 N 个 goroutine 同时调 Close，断言 closeHook 触发次数 == N（无重复）。
func TestPool_Close_ConcurrentIsIdempotent(t *testing.T) {
	const instanceCount = 4

	p := &Pool{
		inits: make(map[*OCR]struct{}),
	}

	closeCount := atomic.Int32{}
	for i := 0; i < instanceCount; i++ {
		// 给每个实例一个真实可清理的 tempDir
		dir, err := os.MkdirTemp("", "ocr-pool-close-test-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })

		o := &OCR{tempDir: dir}
		o.closeHook = func() {
			closeCount.Add(1)
		}
		p.inits[o] = struct{}{}
	}

	const closers = 8
	var wg sync.WaitGroup
	wg.Add(closers)
	for i := 0; i < closers; i++ {
		go func() {
			defer wg.Done()
			if err := p.Close(); err != nil {
				t.Errorf("Close 返回错误: %v", err)
			}
		}()
	}
	wg.Wait()

	// 关键断言：所有实例只被 Close 一次（不是 closers 倍数）
	got := int(closeCount.Load())
	if got != instanceCount {
		t.Errorf("实例 Close 应执行 %d 次（每个实例一次），实际 %d 次", instanceCount, got)
	}
}
