package client

import (
	"context"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// ParallelDimsResult 是并行维度查询的聚合结果。
type ParallelDimsResult[T any] struct {
	Items          []T     // 所有成功维度的 item
	BizErrors      []error // 非 context 取消的业务错误
	ContextErrors  []error // context 取消/超时错误
	CancelledCount int     // 因 ctx 取消/超时而失败的维度数
	FailedCount    int     // 因业务错误而失败的维度数
}

// ParallelDims 对维度列表并发执行 fn，聚合结果并自动分类错误。
//
// 行为：
//   - 跳过 id=0 的汇总维度
//   - 并发上限 = limit（>0）；limit<=0 时默认 = active dims
//   - fn 接收含 errgroup 取消传播的 ctx 和单个 dimension，返回该维度的 items 和 error
//   - 单个维度失败不中断其他维度的执行
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
	allItems := make([]T, 0, len(dims)*10)
	allErrs := make([]error, 0, len(dims))

	for _, dim := range dims {
		if dim.ID == 0 {
			continue
		}
		d := dim
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			items, err := fn(gctx, d)
			if err != nil {
				appendLocked(&mu, &allErrs, err)
				return nil
			}
			if len(items) > 0 {
				appendLocked(&mu, &allItems, items...)
			}
			return nil
		})
	}

	egErr = g.Wait()
	result = &ParallelDimsResult[T]{Items: allItems}

	for _, e := range allErrs {
		switch ClassifyError(e) {
		case CategoryContextCancel, CategoryContextTimeout:
			result.CancelledCount++
			result.ContextErrors = append(result.ContextErrors, e)
		default:
			result.FailedCount++
			result.BizErrors = append(result.BizErrors, e)
		}
	}
	return result, egErr
}
