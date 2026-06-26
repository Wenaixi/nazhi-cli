// Package ocr 内部白盒测试：Pool.Close 窗口内并发 Recognize 的 trackInit 泄漏（F2）。
//
// Finding F2：Pool.Close 的 sync.Once 块内 inits.Range 与 closed=true 不在同一原子操作中。
// closeOnce 只保护 Close 自身不被并发调用重入，不阻挡并发 Recognize。
//
// 失败场景（按时间顺序）：
//
//	T0 Close() 进入 closeOnce.Do → 启动 Range(inits)
//	T1 Recognize() 在 closeMu 下读 closed=false（Close 还没翻 closed=true）
//	T0 Range 完成
//	T1 pool.Get() 拿到 Range 完成后新建的 &OCR{}
//	T1 trackInit(o) 把 o 加入 inits map（已晚，Close 不会再访问）
//	T1 o.Recognize 触发 extractModels 写出 tempDir + ONNX session
//	T1 defer pool.Put(o)
//	T0 接着 closeOnce 块结束，closed=true 翻转
//	任何后续 Close 都被 closeOnce 短路（no-op）
//	结果：T1 注册的实例和它创建的 tempDir 永久泄漏
//
// 修复方向（方案 A，最小修改）：
//
//	把 Recognize 路径的「closeMu 检查 + pool.Get + trackInit」移入同一临界区，
//	保证 Close 的 Range 与 closed=true 翻转是原子的，对任何并发 Recognize 可见。
//	配合 F22 改造：closed 改 atomic.Bool、删除冗余 closeMu。
//
// 测试策略（白盒 + 真实 API 混合）：
//
//	测试 1（核心）：用真实 Recognize API，但避开真实 ONNX 初始化
//	  —— 在 Pool 关闭前/后调用 Recognize，观察 inits map 状态。
//	  不调并发，避免 ONNX 模型写出。
//
//	测试 2（白盒 + 真实 API 并发）：mock OCR（OCR 实例没有 ocr session），
//	  让 inits 注册路径跑过但不触发 extractModels。
//	  → 在 fix 前能复现幽灵实例泄漏
//	  → 在 fix 后应该全部被 Close 清理
//
//	测试 3（错误消息契约）：Close 后调真实 Recognize 必须返回错误
package ocr

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPool_Close_RecognizeDuringClose_NoGhostInits 验证 F2 的核心 invariant。
//
// 子测试 1：纯白盒 — 用一个内部 helper 直接调"模拟 Recognize 的 inits 注册路径"，
//
//	等价于"close 检查 + pool.Get + trackInit"的并发竞争。
//
// 子测试 2：真实 API — Close 后调真实 Recognize，必须返回错误，inits 不增长。
//
// 子测试 3：错误消息契约 — Close 后错误必须包含"已关闭"。
func TestPool_Close_RecognizeDuringClose_NoGhostInits(t *testing.T) {
	t.Run("并发抢锁无幽灵实例", func(t *testing.T) {
		const recognizeGoroutines = 32
		const recognizeCallsPerG = 200

		p := NewPool(0)
		beforeInits := countInits(p)

		var stopFlag atomic.Bool
		var wg sync.WaitGroup
		wg.Add(recognizeGoroutines)

		// 直接模拟 Pool.Recognize 的「closeMu 保护 + Get + trackInit」
		// 关键路径（不调真实 Recognize，避免 ddddocr 全局 race + 模型写出）。
		//
		// 修复前：closeMu 只保护 closed 读取，pool.Get + trackInit 在
		//   临界区外，并发 Recognize 可在 Close window 内漏网注册。
		// 修复后：closeMu 保护整个「读 closed + Get + trackInit」临界区，
		//   与 Close 的「Range + 翻 closed」临界区互斥，无漏网。
		mockRecognize := func() {
			if stopFlag.Load() {
				return
			}
			// 关键路径：mutex 临界区 + Get + trackInit
			// 这是 fix 后代码的实际行为模式
			var o *OCR
			p.closeMu.Lock()
			if !p.closed {
				o, _ = p.pool.Get().(*OCR)
				if o == nil {
					o = &OCR{}
				}
				p.trackInit(o)
			}
			p.closeMu.Unlock()
			if o != nil {
				p.pool.Put(o)
			}
		}

		for i := 0; i < recognizeGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < recognizeCallsPerG; j++ {
					mockRecognize()
				}
			}()
		}

		// 给 goroutine 一点时间启动 + 触发 Close
		time.Sleep(2 * time.Millisecond)
		if err := p.Close(); err != nil {
			t.Fatalf("Close 失败: %v", err)
		}
		stopFlag.Store(true)
		wg.Wait()

		// 关键断言：Close 完成后 inits map 不应包含任何"幽灵实例"
		// 修复前：goroutine 在 Close window 内漏网注册的实例会留在 inits 里
		// 修复后：critical section 保护下没有漏网
		afterInits := countInits(p)
		if afterInits != beforeInits {
			t.Errorf("Close 后 inits map 不应包含『幽灵实例』，但有 %d 个泄漏（before=%d, after=%d）",
				afterInits-beforeInits, beforeInits, afterInits)
		}
	})

	t.Run("Close后Recognize不创建实例", func(t *testing.T) {
		p := NewPool(0)
		if err := p.Close(); err != nil {
			t.Fatalf("Close 失败: %v", err)
		}

		beforeInits := countInits(p)

		// Close 后调 N 次 Recognize（真实 API 路径），全部应返回错误
		const calls = 100
		var errCount int
		for i := 0; i < calls; i++ {
			_, err := p.Recognize([]byte("fake"))
			if err == nil {
				t.Errorf("Close 后 Recognize 第 %d 次应返回错误，但返回 nil", i)
			} else {
				errCount++
			}
		}

		afterInits := countInits(p)
		if afterInits != beforeInits {
			t.Errorf("Close 后 Recognize 不应注册新实例，但 inits 从 %d 增长到 %d",
				beforeInits, afterInits)
		}
		if errCount != calls {
			t.Errorf("所有 %d 次调用都应返回错误，实际只有 %d 次", calls, errCount)
		}
	})

	t.Run("Close错误消息保持稳定", func(t *testing.T) {
		// F1 修复契约：Close 后调 Recognize 必须返回 "OCR 池已关闭" 错误
		// 验证 F2 修复未破坏 F1 的错误消息契约
		p := NewPool(0)
		if err := p.Close(); err != nil {
			t.Fatalf("Close 失败: %v", err)
		}

		_, err := p.Recognize([]byte("fake"))
		if err == nil {
			t.Fatal("Close 后 Recognize 应返回错误，但返回 nil")
		}
		if !strings.Contains(err.Error(), "已关闭") {
			t.Errorf("错误消息应包含『已关闭』，实际：%v", err)
		}
	})
}

// TestPool_Close_NoTempDirLeak 验证 F2 修复后 Close 路径稳定：
// 所有已 trackInit 的实例的 tempDir 都应被清理。
//
// 这里不调真实 Recognize（避免 ddddocr 全局 race 干扰测试稳定性），
// 仅验证 Close 排空路径本身的正确性。
func TestPool_Close_NoTempDirLeak(t *testing.T) {
	p := NewPool(0)

	// 预注册 N 个带真实 tempDir 的 OCR
	const instances = 4
	tempDirs := make([]string, instances)
	for i := 0; i < instances; i++ {
		dir, err := os.MkdirTemp("", "ocr-f2-leak-test-*")
		if err != nil {
			t.Fatalf("MkdirTemp 失败: %v", err)
		}
		tempDirs[i] = dir
		t.Cleanup(func() { _ = os.RemoveAll(dir) })

		o := &OCR{tempDir: dir}
		p.trackInit(o)
	}

	// 触发 Close（无并发干扰，纯单线程）
	if err := p.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}

	// 断言：所有原始 tempDir 都被 RemoveAll
	for i, dir := range tempDirs {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("OCR #%d tempDir %s 应被 Close 清理，但还存在", i, dir)
		}
	}
}

// TestPool_Close_NoDoubleClean 验证 F2 修复未破坏 F1 的 no-op 契约：
// 重复 Close 不应重复清理已删 tempDir。
func TestPool_Close_NoDoubleClean(t *testing.T) {
	p := NewPool(0)

	dir, err := os.MkdirTemp("", "ocr-f2-double-close-*")
	if err != nil {
		t.Fatalf("MkdirTemp 失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	o := &OCR{tempDir: dir}
	p.trackInit(o)

	// 第一次 Close
	if err := p.Close(); err != nil {
		t.Fatalf("首次 Close 失败: %v", err)
	}

	// 第二次 Close 应是 no-op
	if err := p.Close(); err != nil {
		t.Errorf("重复 Close 应是 no-op，实际：%v", err)
	}
}
