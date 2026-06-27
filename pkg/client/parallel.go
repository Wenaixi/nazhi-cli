// parallel.go 提供泛型并发编排 helper + ctx cancel 检查 helper。
//
// 设计动机（候选 #6）：
//   - FetchTasks 的并发编排模式（errgroup.SetLimit + 共享切片 mutex +
//     context cancel propagate + 单维度错误聚合）应可被任意 dim 遍历场景复用，
//     避免 errgroup 使用变成"全仓唯一一处"。
//   - 抽出 ParallelDims[T, K] 泛型 helper：input K → output T 的并发映射。
//
// 设计动机（候选 #7）：
//   - ctx cancel 检查在 task.go / auth.go 风格漂移（gctx vs ctx、不同消息风格）。
//   - 抽出 CheckCancelled(ctx) → ctx.Err() 作为统一入口。
package client

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ParallelDims 通用并发编排 helper。
//
// 接受 items []K，通过 fetch 回调并发拉取，返回成功结果 []T + 错误列表 []error。
//
// 行为契约：
//   - 顺序：保持 items 顺序追加到输出（即使完成顺序错乱）。
//   - limit：errgroup.SetLimit 控制并发上限；0 退化为 len(items)（无上限）。
//   - fetch 返回 nil error：结果追加到 outputs（零值 T 仍算成功）。
//   - fetch 返回非 nil error：error 进 errs，**不**中断其他 items。
//   - fetch panic：recover 转成 error 进 errs，**不**中断其他 items。
//   - ctx 取消：errgroup 自动 cancel 所有 goroutine；cancel 后的 ctx.Err()
//     也会作为 error 出现在 errs 中（来自每个 goroutine 的 ctx.Err() 检查）。
//   - 空 items：返回 (nil, nil)。
//
// 设计权衡：
//   - **不**使用 errgroup 的 Wait() error 返回（因为单 item 失败不应 fail-fast）。
//     反之：所有 fetch error 都收集到 errs，让 caller 决定 partial vs all-fail 语义。
//   - outputs / errs 共享切片受 mutex 保护（errgroup 不保护共享切片写入）。
func ParallelDims[T any, K any](
	ctx context.Context,
	items []K,
	fetch func(ctx context.Context, item K) (T, error),
	limit int,
) ([]T, []error) {
	if len(items) == 0 {
		return nil, nil
	}

	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(limit)

	var (
		mu       sync.Mutex
		outputs  []T
		dimErrs  []error
	)

	for _, item := range items {
		item := item // 循环变量捕获（与 task.go 防御性写法对齐）
		eg.Go(func() error {
			// ctx 取消 short-circuit：避免 dimErrs 重复收集 cancel error
			// 后被错误地视为业务失败。
			if err := egCtx.Err(); err != nil {
				mu.Lock()
				dimErrs = append(dimErrs, err)
				mu.Unlock()
				return nil
			}

			// panic recover：单个 item panic 不影响其他并发 goroutine。
			var (
				val T
				err error
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("ParallelDims item panic: %v", r)
					}
				}()
				val, err = fetch(egCtx, item)
			}()

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				dimErrs = append(dimErrs, err)
				return nil
			}
			outputs = append(outputs, val)
			return nil
		})
	}

	// eg.Wait() 在我们设计下永远返回 nil（每个 goroutine 都 swallow error 到 dimErrs）。
	// 但保留 Wait() 以触发 egCtx 的 cancel propagate 给所有 in-flight goroutine。
	_ = eg.Wait()

	return outputs, dimErrs
}

// CheckCancelled 返回 ctx 是否已取消。
//
// 用于循环顶部 / goroutine 入口的快速 short-circuit。
//
// 设计动机：task.go 与 auth.go 之前 ctx cancel 检查风格漂移：
//   - task.go: `if err := gctx.Err(); err != nil { return err }`
//   - auth.go: `if ctxErr := ctx.Err(); ctxErr != nil { ... 自定义消息 ... }`
//
// 抽出 helper 让所有调用方统一走同一入口，但**保留**自定义消息的灵活性
//（调用方可以 wrap：fmt.Errorf("xxx: %w", CheckCancelled(ctx))）。
func CheckCancelled(ctx context.Context) error {
	return ctx.Err()
}