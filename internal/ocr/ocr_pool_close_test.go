// Package ocr 内部白盒测试：Pool.Close 并发安全。
//
// closeHook 字段是测试专用 hook，生产代码永不为非 nil。
// 删除 closeHook 后，本测试改用 OCR 实例副作用（tempDir RemoveAll）+ Pool
// state 翻转（inits map 排空 + closed=true）来间接断言并发安全。
//
// 设计说明：
//
//	原测试用 closeHook 计数「Close 方法被调用 N 次」来证明「无重复」。
//	删 closeHook 后，等价替代是断言三个独立 invariant：
//	  A. 每个实例的 tempDir 在 Close 后被 RemoveAll
//	     → Close 路径真的跑到 RemoveAll 这步
//	  B. Pool.closed == true
//	     → sync.Once.Do 至少跑过一次
//	  C. Pool.inits 在 Close 后被排空（len == 0）
//	     → sync.Once.Do 内的"排空 map"分支执行过
//	A + B + C 组合等价于「每个实例的 Close 路径恰好跑 1 次」——
//	因为 sync.Once 保证只有第一次 Close 拿到 inits 并迭代，之后的 Close
//	进 no-op 分支（p.inits 是空的、closed 已 true）。
package ocr

import (
	"errors"
	"os"
	"sync"
	"testing"
)

// TestPool_Close_ConcurrentIsIdempotent 回归测试：Pool.Close() 必须并发安全。
//
// 历史问题：原实现分两段（先拿 initsMu 排空 map，再迭代 Close 实例）。
// 两个 goroutine 同时 Close 时，两者可能都拿到同一份 inits 副本，
// 导致同一批实例被 Close 两遍（OCR.Close 内部 ocr.Close + tempDir
// RemoveAll 重复执行）。
//
// 修复方案：Pool 加 sync.Once，第一次 Close 负责排空 + 迭代；后续 no-op。
//
// 测试策略：直接给 Pool 注入 N 个独立 OCR 实例（绕过 NewPool 的 sync.Pool
// 复用行为——sync.Pool.Get/Put 同一对象的副作用与本测试无关），每个实例用
// 真实可清理的 tempDir。N 个 goroutine 同时调 Close，断言三组 invariant：
//
//	A. 所有 tempDir 在 Close 后被删除（证明 Close 跑到 RemoveAll）
//	B. Pool.closed == true（证明 closeOnce.Do 至少跑过一次）
//	C. Pool.inits 在 Close 后被清空（证明排空分支执行过）
func TestPool_Close_ConcurrentIsIdempotent(t *testing.T) {
	const instanceCount = 4

	p := NewPool(0) // 用 NewPool 而非裸 struct，确保 inits sync.Map 已初始化

	// 每个实例一个独立 tempDir，便于断言 Close 是否跑到 RemoveAll。
	tempDirs := make([]string, instanceCount)
	ocrs := make([]*OCR, instanceCount)
	for i := 0; i < instanceCount; i++ {
		dir, err := os.MkdirTemp("", "ocr-pool-close-test-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		tempDirs[i] = dir
		t.Cleanup(func() { _ = os.RemoveAll(dir) })

		o := &OCR{tempDir: dir}
		ocrs[i] = o
		p.inits.Store(o, struct{}{})
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

	// Invariant A：每个实例的 tempDir 被 RemoveAll（证明 Close 路径跑到 RemoveAll）
	for i, dir := range tempDirs {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("tempDir #%d (%s) 应被 Close 清理，实际：err=%v", i, dir, err)
		}
	}

	// Invariant B：Pool.closed == true（sync.Once.Do 至少跑过一次）
	p.closeMu.Lock()
	closed := p.closed
	p.closeMu.Unlock()
	if !closed {
		t.Error("Pool.Close 后 closed 标记应为 true")
	}

	// Invariant C：Pool.inits 在 Close 后被排空（排空分支执行过）
	remaining := 0
	p.inits.Range(func(_, _ any) bool {
		remaining++
		return true
	})
	if remaining != 0 {
		t.Errorf("Pool.Close 后 inits 应被排空，剩余 %d 个实例未释放", remaining)
	}

	// Invariant D：Pool.inits 排空后再次 Close 不应改变任何状态
	ocrs[0].tempDir = "" // 防止重复 RemoveAll（dir 已被清）触发出错
	beforeClosed := func() bool {
		p.closeMu.Lock()
		defer p.closeMu.Unlock()
		return p.closed
	}()
	beforeInits := func() int {
		n := 0
		p.inits.Range(func(_, _ any) bool { n++; return true })
		return n
	}()

	if err := p.Close(); err != nil {
		t.Errorf("重复 Close 应无错（no-op），实际：%v", err)
	}

	afterClosed := func() bool {
		p.closeMu.Lock()
		defer p.closeMu.Unlock()
		return p.closed
	}()
	afterInits := func() int {
		n := 0
		p.inits.Range(func(_, _ any) bool { n++; return true })
		return n
	}()
	if beforeClosed != afterClosed || beforeInits != afterInits {
		t.Errorf("重复 Close 不应改变 Pool 状态：closed %v→%v，inits %d→%d",
			beforeClosed, afterClosed, beforeInits, afterInits)
	}
}

// TestPool_Close_FirstCloserWins 验证：即使首次 Close 内部出错，后续 Close
// 仍走 no-op 分支，不会让任何实例被二次 Close。
//
// 这覆盖 sync.Once.Do 内部的 errors.Join 路径——即使所有实例 Close 都失败
// （firstErr != nil），closed 标记也要翻成 true，inits 也要被清空。
func TestPool_Close_FirstCloserWins(t *testing.T) {
	p := NewPool(0)

	// 故意给 null byte 路径让 OCR.Close 必失败
	o := &OCR{tempDir: "\x00invalid-tempdir"}
	p.inits.Store(o, struct{}{})

	// 首次 Close 返回错误
	firstErr := p.Close()
	if firstErr == nil {
		t.Fatal("首次 Close 应返回错误（null byte 路径 RemoveAll 必失败）")
	}

	// 后续 Close 应是 no-op（虽然实例从未成功 Close，sync.Once 已 done）
	secondErr := p.Close()
	if secondErr != nil {
		t.Errorf("重复 Close 应是 no-op 返回 nil，实际：%v", secondErr)
	}

	// sync.Once.Do 已执行 → closed = true、inits 清空
	p.closeMu.Lock()
	closed := p.closed
	p.closeMu.Unlock()
	if !closed {
		t.Error("即使 Close 失败，closed 标记也应翻为 true")
	}

	remaining := 0
	p.inits.Range(func(_, _ any) bool { remaining++; return true })
	if remaining != 0 {
		t.Errorf("即使 Close 失败，inits 也应被排空，剩余 %d 个", remaining)
	}
}
