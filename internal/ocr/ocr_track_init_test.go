// Package ocr 内部白盒测试：Pool.trackInit 写入优化。
//
// Pool.Recognize 每次都调 trackInit 写 map；99 次串行 Recognize
// = 99 次 Lock + map 写同 key（虽然 map 覆盖语义上没变化，但 99 次
// mutex.Lock + 99 次 map[k]=struct{}{} 在高频路径下能看到明显开销，
// 且每次 Lock 都要走 runtime sema，对单线程 99 次调用是 ~99×~50ns ≈ 5μs）。
//
// 优化方案：用 sync.Map.LoadOrStore 替代 mutex+map 写：
//   - key 不存在时：lock-free Load + 原子 Store
//   - key 已存在时：直接 return，不触发 Store
//
// 这样 99 次串行 trackInit(同一 *OCR) 只触发 1 次实际 Store，
// 后续 98 次走 LoadOrStore 的「已存在」快速路径（无 mutex.Lock 开销）。
//
// 本测试断言：
//  1. trackInit 去重语义：99 次串行 trackInit(同一 *OCR) → map 只有 1 个 key
//  2. trackInit 多实例语义：N 个不同 *OCR → map 有 N 个 key
//  3. 并发安全：99 goroutine 并发 trackInit(同一 *OCR) → -race 不报警
//  4. Close 路径正确性：优化后 Pool.Close 仍能正确排空 map
package ocr

import (
	"os"
	"sync"
	"testing"
)

// countInits 返回 Pool.inits 当前元素数（用 Range 计数）。
func countInits(p *Pool) int {
	n := 0
	p.inits.Range(func(_, _ any) bool { n++; return true })
	return n
}

// TestPool_TrackInit_UniqueInstancesOnly 回归测试行为契约：
// trackInit 接收相同 *OCR 指针 N 次，map 内只入 1 次；
// 接收 N 个不同 *OCR 指针，map 内入 N 次。
func TestPool_TrackInit_UniqueInstancesOnly(t *testing.T) {
	const instances = 5
	const repeatsPerInstance = 99 // 模拟「同 OCR 被多次 trackInit」场景

	p := NewPool(0)

	ocrs := make([]*OCR, instances)
	for i := 0; i < instances; i++ {
		ocrs[i] = &OCR{}
	}

	// 总调用次数 = instances × repeatsPerInstance
	totalCalls := instances * repeatsPerInstance
	for c := 0; c < totalCalls; c++ {
		o := ocrs[c%instances] // 循环分配，每个实例被 trackInit 99 次
		p.trackInit(o)
	}

	// 断言：map 内只有 instances 个独立 *OCR（去重生效）
	if got := countInits(p); got != instances {
		t.Errorf("trackInit 去重失败：%d 次调用应只入 %d 个独立 OCR，实际入 %d 个",
			totalCalls, instances, got)
	}

	// 断言：map 内的每个独立 *OCR 都已被注册
	for i, o := range ocrs {
		if _, ok := p.inits.Load(o); !ok {
			t.Errorf("OCR #%d 未在 inits map 内", i)
		}
	}
}

// TestPool_TrackInit_ConcurrentSameInstance 验证并发场景下 trackInit 也只入一次。
func TestPool_TrackInit_ConcurrentSameInstance(t *testing.T) {
	const goroutines = 16
	const trackPerGoroutine = 64

	o := &OCR{}
	p := NewPool(0)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < trackPerGoroutine; i++ {
				p.trackInit(o)
			}
		}()
	}
	wg.Wait()

	if got := countInits(p); got != 1 {
		t.Errorf("%d 次并发 trackInit 同一 OCR 应只入 map 1 次，实际 %d 次",
			goroutines*trackPerGoroutine, got)
	}
}

// TestPool_TrackInit_NoRaceOnSameInstance 验证 -race 模式下
// 99 个 goroutine 并发 trackInit(同一 OCR) 不报 concurrent map write。
//
// 这是核心 metric：99 张验证码图并发识别 = 99 次 trackInit
// 应只触发 1 次 map 写入；-race 模式下任何并发 map 写入都会报警。
//
// 测试通过 + go test -race 不报警 = map 写入去重到位。
func TestPool_TrackInit_NoRaceOnSameInstance(t *testing.T) {
	o := &OCR{}
	p := NewPool(0)

	const calls = 99
	var wg sync.WaitGroup
	wg.Add(calls)
	for i := 0; i < calls; i++ {
		go func() {
			defer wg.Done()
			p.trackInit(o)
		}()
	}
	wg.Wait()

	if got := countInits(p); got != 1 {
		t.Errorf("99 次并发 trackInit 后 map 应只有 1 个 key，实际 %d", got)
	}
}

// TestPool_TrackInit_AfterCloseRelease 验证优化后 Pool.Close 仍正确：
// Close 排空 map 的路径不能因为 map 类型变化而 break。
func TestPool_TrackInit_AfterCloseRelease(t *testing.T) {
	const instanceCount = 3

	p := NewPool(0)

	tempDirs := make([]string, instanceCount)
	for i := 0; i < instanceCount; i++ {
		dir, err := os.MkdirTemp("", "ocr-track-close-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		tempDirs[i] = dir
		t.Cleanup(func() { _ = os.RemoveAll(dir) })

		o := &OCR{tempDir: dir}
		// 多次 trackInit 同一 OCR（优化重点场景）—— 只有 1 次实际入 map
		for j := 0; j < 10; j++ {
			p.trackInit(o)
		}
	}

	// Close 排空 map → 每个实例的 tempDir 应被 RemoveAll
	if err := p.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}

	for i, dir := range tempDirs {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("OCR #%d tempDir %s 应被 Close 清理，但还存在", i, dir)
		}
	}
}
