// parallel_test.go ParallelDims[T] + CheckCancelled 单元测试。
//
// 测试目标（候选 #6 / #7）：
//   - ParallelDims 串行执行（limit=1）
//   - ParallelDims 并发执行（limit=4，验证 in-flight 峰值 ≈ 4）
//   - ParallelDims ctx 取消 short-circuit
//   - ParallelDims 分离 context 错误与业务错误到 errs
//   - ParallelDims 空 items 返回零值
//   - ParallelDims panic 不影响其他 item（recover 到 errs）
//   - CheckCancelled alive ctx 返回 nil
//   - CheckCancelled cancelled ctx 返回 context.Canceled
package client

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestParallelDims_LimitOneIsSerial 验证 limit=1 时并发降级为串行：
// in-flight 峰值应恒等于 1（无重叠执行）。
func TestParallelDims_LimitOneIsSerial(t *testing.T) {
	var (
		inFlight     atomic.Int32
		peakInFlight atomic.Int32
	)

	items := []int{1, 2, 3, 4, 5}
	out, errs := ParallelDims[int](context.Background(), items, func(ctx context.Context, v int) (int, error) {
		cur := inFlight.Add(1)
		// CAS 记录峰值
		for {
			old := peakInFlight.Load()
			if cur <= old || peakInFlight.CompareAndSwap(old, cur) {
				break
			}
		}
		// 持有 20ms，确保如果有并发会发生重叠
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		return v * 10, nil
	}, 1) // limit=1: 串行

	if len(errs) > 0 {
		t.Fatalf("期望无错误，实际: %v", errs)
	}
	if got := peakInFlight.Load(); got != 1 {
		t.Errorf("limit=1 应串行执行，峰值 in-flight=%d（期望 1）", got)
	}
	if len(out) != len(items) {
		t.Errorf("输出数量不匹配：期望 %d，实际 %d", len(items), len(out))
	}
}

// TestParallelDims_LimitFourMaxInflightFour 验证 limit=4 时并发度受控：
// 8 个 items × limit=4，应观察到 in-flight 峰值 ≤ 4。
func TestParallelDims_LimitFourMaxInflightFour(t *testing.T) {
	const (
		limit      = 4
		itemCount  = 16 // 4 × limit，确保 limit 真的会触顶
		perItemDur = 30 * time.Millisecond
	)
	var (
		inFlight     atomic.Int32
		peakInFlight atomic.Int32
	)

	items := make([]int, itemCount)
	for i := range items {
		items[i] = i
	}

	out, errs := ParallelDims[int](context.Background(), items, func(ctx context.Context, v int) (int, error) {
		cur := inFlight.Add(1)
		for {
			old := peakInFlight.Load()
			if cur <= old || peakInFlight.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(perItemDur)
		inFlight.Add(-1)
		return v, nil
	}, limit)

	if len(errs) > 0 {
		t.Fatalf("期望无错误，实际: %v", errs)
	}
	if got := peakInFlight.Load(); got > int32(limit) {
		t.Errorf("limit=%d 时 in-flight 峰值=%d 超限", limit, got)
	}
	if got := peakInFlight.Load(); got < 2 {
		t.Errorf("limit=%d 应观察到至少 2 路并发，实际峰值=%d（可能 limit 未生效）", limit, got)
	}
	if len(out) != itemCount {
		t.Errorf("输出数量不匹配：期望 %d，实际 %d", itemCount, len(out))
	}
}

// TestParallelDims_CtxCancelShortCircuits 验证 ctx 取消时：
//   - 后续未启动的 items 跳过（fetch 不被调用）
//   - 已启动的 fetch 收到 cancel 信号
//   - 返回的 errs 包含 context.Canceled/DeadlineExceeded
func TestParallelDims_CtxCancelShortCircuits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var started atomic.Int32

	items := make([]int, 50) // 多到不可能全部启动
	for i := range items {
		items[i] = i
	}

	out, errs := ParallelDims[int](ctx, items, func(ctx context.Context, v int) (int, error) {
		started.Add(1)
		// 第一个完成前取消
		if started.Load() == 1 {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}
		// 监听 ctx
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return v, nil
		}
	}, 2)

	if started.Load() >= int32(len(items)) {
		t.Errorf("ctx 取消后应 short-circuit，但所有 %d 个 item 都启动了", len(items))
	}
	if len(errs) == 0 {
		t.Error("期望至少一个 errs（context cancel），实际 0")
	}
	// errs 中应至少包含一个 context.Canceled
	hasCtxErr := false
	for _, e := range errs {
		if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			hasCtxErr = true
			break
		}
	}
	if !hasCtxErr {
		t.Errorf("errs 应包含 context 错误，实际: %v", errs)
	}
	t.Logf("ctx 取消：started=%d/%d, out=%d, errs=%d", started.Load(), len(items), len(out), len(errs))
}

// TestParallelDims_SeparatesBizAndCtxErrors 验证 fetch 返回的业务错误被收集，
// ctx 取消错误也被收集（不丢失任何错误信号）。
func TestParallelDims_SeparatesBizAndCtxErrors(t *testing.T) {
	bizErr := errors.New("business boom")
	items := []int{1, 2, 3, 4}

	out, errs := ParallelDims[int](context.Background(), items, func(ctx context.Context, v int) (int, error) {
		switch v {
		case 1:
			return 0, bizErr
		case 2:
			return 0, context.Canceled
		case 3:
			return v, nil
		case 4:
			return 0, bizErr
		}
		return v, nil
	}, 4)

	if len(out) != 1 || out[0] != 3 {
		t.Errorf("期望 out=[3]，实际 %v", out)
	}
	if len(errs) != 3 {
		t.Fatalf("期望 3 个错误，实际 %d：%v", len(errs), errs)
	}
	// 验证 bizErr 与 ctx 错误都存在
	bizCount := 0
	ctxCount := 0
	for _, e := range errs {
		if errors.Is(e, bizErr) {
			bizCount++
		}
		if errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			ctxCount++
		}
	}
	if bizCount != 2 {
		t.Errorf("期望 2 个 bizErr，实际 %d", bizCount)
	}
	if ctxCount != 1 {
		t.Errorf("期望 1 个 ctx 错误，实际 %d", ctxCount)
	}
}

// TestParallelDims_EmptyItems 验证空 items 输入返回零值结果且无错误。
func TestParallelDims_EmptyItems(t *testing.T) {
	out, errs := ParallelDims[int](context.Background(), nil, func(ctx context.Context, v int) (int, error) {
		t.Fatal("fetch 不应被调用")
		return v, nil
	}, 4)

	if len(out) != 0 {
		t.Errorf("期望空输出，实际 %v", out)
	}
	if len(errs) != 0 {
		t.Errorf("期望无错误，实际 %v", errs)
	}
}

// TestParallelDims_PanicRecoveredToErrs 验证 fetch panic 时：
//   - panic 被 recover 转成 error 进 errs
//   - 不影响其他 item 继续执行
func TestParallelDims_PanicRecoveredToErrs(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	var successCount atomic.Int32
	out, errs := ParallelDims[int](context.Background(), items, func(ctx context.Context, v int) (int, error) {
		if v == 3 {
			panic("intentional panic for test")
		}
		successCount.Add(1)
		return v, nil
	}, 2)

	if got := successCount.Load(); got != int32(len(items)-1) {
		t.Errorf("panic 不应影响其他 item：期望 %d 成功，实际 %d", len(items)-1, got)
	}
	if len(errs) != 1 {
		t.Fatalf("期望 1 个 panic error，实际 %d：%v", len(errs), errs)
	}
	if !contains(errs[0].Error(), "panic") {
		t.Errorf("error 应含 'panic'，实际：%v", errs[0])
	}
	if len(out) != int(successCount.Load()) {
		t.Errorf("out 数量应等于成功数：期望 %d，实际 %d", successCount.Load(), len(out))
	}
}

// ─── CheckCancelled 测试 ───

// TestCheckCancelled_AliveReturnsNil 验证未取消的 ctx 返回 nil。
func TestCheckCancelled_AliveReturnsNil(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := CheckCancelled(ctx); err != nil {
		t.Errorf("alive ctx 应返回 nil，实际: %v", err)
	}
}

// TestCheckCancelled_CancelledReturnsCanceled 验证已取消的 ctx 返回 context.Canceled。
func TestCheckCancelled_CancelledReturnsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := CheckCancelled(ctx)
	if err == nil {
		t.Fatal("cancelled ctx 应返回非 nil error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("应返回 context.Canceled，实际: %v", err)
	}
}

// TestCheckCancelled_DeadlineReturnsDeadlineExceeded 验证超时 ctx 返回 context.DeadlineExceeded。
func TestCheckCancelled_DeadlineReturnsDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	err := CheckCancelled(ctx)
	if err == nil {
		t.Fatal("deadline ctx 应返回非 nil error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("应返回 context.DeadlineExceeded，实际: %v", err)
	}
}

// contains 简单字符串包含辅助。
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}