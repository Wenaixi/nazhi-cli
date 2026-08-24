package client

import (
	"context"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// ParallelDimsResult 是并行维度查询的聚合结果。
type ParallelDimsResult[T any] struct {
	Items          []T     // 所有成功维度的 item（按维度声明顺序拼接）
	BizErrors      []error // 非 context 取消的业务错误
	ContextErrors  []error // context 取消/超时错误
	CancelledCount int     // 因 ctx 取消/超时而失败的维度数
	FailedCount    int     // 因业务错误而失败的维度数
}

// ParallelDims 对维度列表并发执行 fn，聚合结果并自动分类错误。
//
// 行为：
//   - 跳过 id=0 的汇总维度
//   - 并发上限 = limit；limit<=0 时按 1（串行）执行
//   - fn 接收含 errgroup 取消传播的 ctx 和单个 dimension，返回该维度的 items 和 error
//   - 单个维度失败不中断其他维度的执行
//   - Items 按维度声明顺序拼接（与完成顺序无关），保证同输入同输出；
//     FetchTasksJSON 的 results[idx] 保序策略与本实现对齐
//
// 返回的 ParallelDimsResult 包含聚合后的 items、分类后的错误列表和计数。
// egErr 是 errgroup.Wait() 返回的错误（当 goroutine 直接 return err 时触发，
// 通常只传递 context 取消信号）。
//
// ponytail: 只做 fan-out + collect + 分类，调用方负责最终 error 包装。
func ParallelDims[T any](ctx context.Context, dims []types.Dimension, limit int, fn func(context.Context, types.Dimension) ([]T, error)) (result *ParallelDimsResult[T], egErr error) {
	lim := limit
	if lim <= 0 {
		lim = 1
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(lim)

	var mu sync.Mutex
	allErrs := make([]error, 0, len(dims))

	// 按活跃维度索引预分配槽位：每个 goroutine 写自己的下标，无竞态；
	// Wait 后按声明顺序展平，输出与调度顺序解耦（确定性契约）。
	type dimBatch struct {
		items []T
	}
	active := make([]types.Dimension, 0, len(dims))
	for _, dim := range dims {
		if dim.ID == 0 {
			continue // 跳过汇总维度
		}
		active = append(active, dim)
	}
	batches := make([]dimBatch, len(active))

	for idx, dim := range active {
		i := idx
		d := dim
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				// 预取消的维度也计入 allErrs：混合失败场景下调用方的
				// eg.Err 分支仍能拿到已收集的业务错误诊断（不吞错）。
				appendLocked(&mu, &allErrs, err)
				return err
			}
			items, err := fn(gctx, d)
			if err != nil {
				appendLocked(&mu, &allErrs, err)
				return nil
			}
			if len(items) > 0 {
				batches[i].items = items // 仅写自己的槽位，无需锁
			}
			return nil
		})
	}

	egErr = g.Wait()

	allItems := make([]T, 0, len(active)*10)
	for i := range batches {
		allItems = append(allItems, batches[i].items...)
	}
	result = &ParallelDimsResult[T]{Items: allItems}

	for _, e := range allErrs {
		switch ClassifyError(e) { //nolint:exhaustive
		case CategoryContextCancel, CategoryContextTimeout:
			result.CancelledCount++
			result.ContextErrors = append(result.ContextErrors, e)
		case CategoryNetworkTimeout, CategoryBusinessError:
			result.FailedCount++
			result.BizErrors = append(result.BizErrors, e)
		default:
			result.FailedCount++
			result.BizErrors = append(result.BizErrors, e)
		}
	}
	return result, egErr
}
