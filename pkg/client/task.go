package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"os"
	"runtime/debug"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// fetchTasksConcurrentLimit 是 FetchTasks 并发拉取维度的上限。
//
// 设计权衡：业务系统实际维度数通常 ≤ 20，单次 FetchTasks 并发度受维度数封顶，
// 远低于 DoS 阈值。限制 = min(len(dimensions), 8) 平衡 wall time 与服务端压力：
//   - 8 路并发足够让 20 维度在 ~3 RTT 内完成（vs 串行 20 RTT）
//   - 不会因下游抖动放大熔断风险
//
// 如未来业务接口维度数 > 50，可考虑调到此常量或暴露为 Client 字段。
const fetchTasksConcurrentLimit = 8

// appendLocked 在 mu 锁内安全地追加 items 到 slice。
//
// 消除 FetchTasks goroutine 闭包内重复的 mu.Lock + append + mu.Unlock 模式。
// 使用 *[]T 而非 []T 返回值，避免调用方忽略 slice header realloc 的 bug：
// append 在容量不足时会分配新底层数组，调用方必须用返回值回写原变量。
// 传入指针让 helper 直接修改调用方的 slice header，调用方无需重新赋值。
//
// 泛型支持单元素追加（dimErrs = appendLocked(&mu, &dimErrs, err)）
// 和变长追加（allTasks = appendLocked(&mu, &allTasks, tasks...)）。
func appendLocked[T any](mu *sync.Mutex, slice *[]T, items ...T) {
	mu.Lock()
	*slice = append(*slice, items...)
	mu.Unlock()
}

// isContextError 判断是否为上下文取消或超时错误。
func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// fetchDimensions 拉取任务维度列表（FetchTasks / GetDimensions 共用）。
// 内部包含 session 预热 + 响应解码，错误信息前缀由 caller 决定。
func (c *Client) fetchDimensions(ctx context.Context, token string, errPrefix string) ([]types.Dimension, error) {
	dims, err := doBizGetDecode[[]types.Dimension](c, ctx, token, errPrefix, "/api/studentCircleNew/getDimensions",
		func(resp types.UnifiedResponse) (*[]types.Dimension, error) {
			v, err := types.DecodeDataList[types.Dimension](resp)
			if err != nil {
				return nil, err
			}
			return &v, nil
		},
	)
	if err != nil {
		return nil, err
	}
	return *dims, nil
}

// FetchTasks 拉取目标平台全部维度的任务列表。
// 内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。
//
// 并发拉取：多个维度的 getCircleStatistics 通过 errgroup 并发执行，
// 并发上限 = min(len(dimensions), fetchTasksConcurrentLimit)。
// 既享受并发提速（20 维度 ≈ 3 RTT vs 串行 20 RTT），
// 又防止 > 50 维度的业务接口把服务端打爆（无限制 goroutine fan-out 风险）。
//
// 单个维度失败时通过 c.logDebug() 记录（不会中断整体拉取），
// 调用方可通过 client.WithLogger() 注入自定义 logger 捕获详细错误。
func (c *Client) FetchTasks(ctx context.Context, token string) ([]types.Task, error) {
	dimensions, err := c.fetchDimensions(ctx, token, "FetchTasks getDimensions")
	if err != nil {
		return nil, err
	}

	headers := c.bizHeaders(token)

	// 3. 并发遍历每个维度获取任务统计，上限由 errgroup.SetLimit 守护。
	// 用 errgroup 替代裸 WaitGroup + channel：
	//   - SetLimit 自动阻塞后续 goroutine 启动直到有空槽
	//   - 简化收集器：直接 g.Go() 内部写共享切片（受同一 errgroup 同步保护）
	limit := len(dimensions)
	if limit > fetchTasksConcurrentLimit {
		limit = fetchTasksConcurrentLimit
	}
	if limit == 0 {
		limit = 1
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	// 每个 goroutine 写各自的本地切片，errgroup 不保护共享切片写入，
	// 所以最后在主线程用 mutex 串行合并，避免 race。
	var mu sync.Mutex

	// F7: 预分配切片容量，避免多次扩容。
	// dimErrs 最多 len(dimensions) 个；allTasks 平均每维度 ~5-10 个任务。
	dimCount := len(dimensions)
	allTasks := make([]types.Task, 0, dimCount*10)
	dimErrs := make([]error, 0, dimCount)
	for _, dim := range dimensions {
		// 跳过"全部"维度（id=0），它只是汇总
		if dim.ID == 0 {
			continue
		}
		g.Go(func() error {
			// context 取消后直接 propagate，
			// 防止 cancel 被 dimErrs 吞掉后包装为 ErrBusinessRejected，
			// 调用方无法区分完整成功与 cancel 截断。
			if err := gctx.Err(); err != nil {
				return err
			}
			tasks, dimErr := c.fetchTasksForDimensionSafe(gctx, dim, headers)
			if dimErr != nil {
				appendLocked(&mu, &dimErrs, dimErr)
				// 不返回 error：保留"单维度失败不影响其他维度"语义，
				// 失败信息通过 dimErrs 聚合后整体 propagate。
				return nil
			}
			if tasks == nil {
				return nil
			}
			appendLocked(&mu, &allTasks, tasks...)
			return nil
		})
	}
	// errgroup 因 context 取消返回 error 时：
	//
	//   - 全部失败（无 partial tasks）：裸包装 ErrRetryable 让 errors.Is 识别可重试语义
	//   - 部分失败（有 partial tasks）：双包 ErrBusinessRejected（让 cmd 层 envelope 识别
	//     partial 状态） + ErrRetryable（让 errors.Is 识别 cancel 重试）
	//   - 业务错误（非 ctx error）：保持原包装
	//
	// F2.2 修复：取消时的两条路径（纯 cancel + 混合）都让 errors.Is(err, ErrRetryable)
	// 命中；保留 partial tasks 的同时让 cmd 层 envelope 输出 retryable 信号。
	if err := g.Wait(); err != nil {
		if isContextError(err) {
			if len(allTasks) > 0 {
				return allTasks, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
					ErrBusinessRejected,
					fmt.Errorf("%w: %w", ErrRetryable, err))
			}
			return nil, fmt.Errorf("%w: FetchTasks 全部维度因 context 取消失败: %w", ErrRetryable, err)
		}
		return nil, fmt.Errorf("FetchTasks 并发拉取失败: %w", err)
	}

	// partial failures 聚合，区分全失败与部分失败。
	//
	// B11 修复：区分 context 取消错误与业务错误。
	// context.Canceled/DeadlineExceeded 不应包装为 ErrBusinessRejected，
	// 否则调用方通过 errors.Is(err, ErrBusinessRejected) 判定时会误判
	// 为业务拒绝，而非 context 截断，导致无法正确区分「完整成功」与
	// 「cancel 截断」两种语义。
	//
	// 有 partial tasks 时仍在 ctx cancel 路径
	// 包装 ErrBusinessRejected。
	// 动机：cmd 层（task_list.go）用 errors.Is(err, ErrBusinessRejected)
	// 判断是否输出 partial envelope；若裸返回 context error 则全失败路径走
	// printError，丢 partial tasks。有成功数据时包装 ErrBusinessRejected
	// 让 cmd 层 envelope 可识别；无成功数据时裸返回（调用方更关心 cancel 根因）。
	//
	// 分离逻辑：
	//   - context 错误 + 无 partial tasks → 裸 errors.Join 返回
	//   - context 错误 + 有 partial tasks → 包装 ErrBusinessRejected
	//   - 业务错误 → 保持原样包装（全失败/部分失败分支不变）
	//
	// G5 修复：不再静默丢弃 context cancel 信息，保留 cancelledCount 让调用方可
	// 感知「有 Y 个维度因 ctx cancel 失败」，区分「应重试」与「业务错误」。
	if len(dimErrs) > 0 {
		// 将 context 取消错误分离出来，但保留维度计数
		var bizErrs []error
		var ctxErrs []error
		var cancelledCount int
		for _, de := range dimErrs {
			if isContextError(de) {
				cancelledCount++
				ctxErrs = append(ctxErrs, de)
				continue
			}
			bizErrs = append(bizErrs, de)
		}
		// 取消信号占位不进 bizErrs（避免 failedCount 含占位虚高 1），
		// 但仍 join 进 joined 保留信号——cmd 层仍能感知 cancel 数。
		//
		// F2.1 修复：用 %w 包装 ErrRetryable，让 SDK 用户能 errors.Is 识别
		//「context 取消导致的失败」并触发重试。原裸 fmt.Errorf 只能字符串匹配。
		var cancelPlaceholder error
		if cancelledCount > 0 {
			cancelPlaceholder = fmt.Errorf("%w: %d 个维度因 context 取消而失败", ErrRetryable, cancelledCount)
		}

		// 仅有 context 取消错误
		if len(bizErrs) == 0 && cancelledCount > 0 {
			joined := errors.Join(append(ctxErrs, cancelPlaceholder)...)
			if len(allTasks) == 0 {
				// 无 partial tasks：裸返回，不包装 ErrBusinessRejected
				return nil, joined
			}
			// 有 partial tasks：包装 ErrBusinessRejected 让 cmd 层 envelope 识别
			return allTasks, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
				ErrBusinessRejected, joined)
		}

		// 有真正的业务错误，保持原 ErrBusinessRejected 包装语义。
		// 占位不进 bizErrs，所以 failedCount 仅算真业务失败维度。
		joined := errors.Join(append(append(bizErrs, ctxErrs...), cancelPlaceholder)...)
		failedCount := len(bizErrs)

		if len(allTasks) == 0 {
			// 全维度业务失败
			return nil, fmt.Errorf("%w: FetchTasks 全部 %d 个维度均失败: %w",
				ErrBusinessRejected, failedCount, joined)
		}

		// 部分维度失败：仍有成功任务可用，附带 partial failure 错误信号
		return allTasks, fmt.Errorf("%w: FetchTasks %d 个维度部分失败: %w",
			ErrBusinessRejected, failedCount, joined)
	}

	return allTasks, nil
}

// fetchTasksForDimension 拉取单个维度的任务列表并注入维度名称。
//
// 错误处理（F-GroupD-F 修复）：
//   - HTTP / 网络错误、响应解析错误、任务列表解析错误：return error，
//     由 FetchTasks errgroup 聚合后 propagate，**不**静默吞咽。
//     调用方通过 dimErrs 区分全失败与部分失败。
//   - 业务错误（code != 1）：return error，由 FetchTasks errgroup 聚合后
//     包装为 ErrBusinessRejected propagate。
//
// 关键设计：之前网络/解析错误走 best-effort (nil, nil) 导致调用方无法区分
// 「空数据」与「网络错误」，聚合时 dimErrs 接收不到错误。改为 propagate error
// 后，dimErrs 能正确收集所有非上下文取消的维度错误。
// 改用命名返回值 (tasks []types.Task, err error) 以便
// fetchTasksForDimensionSafe 的 defer recover 能通过闭包赋值捕获 panic。
// 非公有方法改变签名不破坏兼容性。
func (c *Client) fetchTasksForDimension(ctx context.Context, dim types.Dimension, headers map[string]string) (tasks []types.Task, err error) {
	// 上下文取消（Canceled/DeadlineExceeded）直接 propagate，
	// 不吞掉走 best-effort——调用方需要知道 context 信号已触发，才能正确区分
	// 「真空数据」与「被取消」。
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 说明：int64 参数纯数字，直接 strconv.FormatInt 拼接 URL 安全，
	// 无需 URL 编码（数字不包含特殊字符）。如需未来扩展为字符串参数，
	// 应改用 url.Values.Encode()。
	statURL := c.bizURL("/api/studentCircleNew/getCircleStatistics") + "?dimensionId=" + strconv.FormatInt(dim.ID, 10)
	statBody, err := c.httpDo(ctx, http.MethodGet, statURL, nil, headers, "")
	if err != nil {
		if isContextError(err) {
			return nil, err // 上下文取消应 propagate，不做 best-effort 吞没
		}
		c.logDebug("FetchTasks 维度 %d(%s) 请求失败: %v", dim.ID, dim.Name, err)
		return nil, err // propagate 网络错误到 dimErrs，不再静默吞咽
	}

	statResp, err := types.DecodeResponse(statBody)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 响应解析失败: %v", dim.ID, dim.Name, err)
		return nil, err // propagate 解析错误到 dimErrs，不再静默吞咽
	}
	if statResp.Code != 1 {
		// F-GroupD-F：业务错误 propagate，不再静默。
		msg := types.DerefOr(statResp.Msg, "")
		return nil, fmt.Errorf("%w: 维度 %d(%s) 业务错误: code=%d msg=%s", ErrBusinessRejected, dim.ID, dim.Name, statResp.Code, msg)
	}

	tasks, err = types.DecodeDataList[types.Task](statResp)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 任务解析失败: %v", dim.ID, dim.Name, err)
		return nil, err // propagate 任务列表解析错误到 dimErrs，不再静默吞咽
	}

	for i := range tasks {
		tasks[i].DimensionName = dim.Name
	}
	return tasks, nil
}

// fetchTasksForDimensionSafe 是 fetchTasksForDimension 的 panic-safe 包装。
//
// errgroup.Go 闭包内无 panic recover 时，nil deref 或
// 第三方库 panic 会逃逸到 runtime → 进程崩溃 → g.Wait() 永不返回。
// 此 helper 在维度粒度捕获 panic，把它当业务错误记录到 dimErrs，
// 防止单个维度的 panic 影响其他维度的并发拉取。
//
// panic 信息：包含 dim.ID + dim.Name 便于排查（panic 路径无法
// 依赖 errgroup 自带的 nil-safe 包装，必须自己构建可读错误）。
//
// 错误链保留（F10.1）：recover() 返回的是 any，r 是 error 时走 %w
// 保留 chain，让 SDK 用户能用 errors.Is 识别 panic 根因（典型场景：
// mock 误实现 panic(errors.New("xxx")) → 调试时能直接定位根 error）。
func (c *Client) fetchTasksForDimensionSafe(ctx context.Context, dim types.Dimension, headers map[string]string) (tasks []types.Task, err error) {
	defer func() {
		if r := recover(); r != nil {
			os.Stderr.Write(debug.Stack())
			tasks = nil
			err = wrapPanicAsErr(dim, r)
		}
	}()
	return c.fetchTasksForDimension(ctx, dim, headers)
}

// wrapPanicAsErr 把 recover() 拿到的 any 转成可读 error。
//
// r 是 error：走 %w 保留 chain（errors.Is 可穿透命中根因）。
// r 不是 error（string / struct / runtime.nilError 等）：走 %v 兜底。
// r == nil：返回明确 error，避免 nil 走调用链误导调用方。
//
// ponytail：抽出来让 fetchTasksForDimensionSafe defer 闭包保持 3 行内，
// 同时让「r 是 error / 不是 error / nil」三条分支 100% 可测，
// 不污染 fetchTasksForDimension 加测试钩子。
func wrapPanicAsErr(dim types.Dimension, r any) error {
	switch v := r.(type) {
	case nil:
		return fmt.Errorf("维度 %d(%s) panic: <nil>", dim.ID, dim.Name)
	case error:
		return fmt.Errorf("维度 %d(%s) panic: %w", dim.ID, dim.Name, v)
	default:
		return fmt.Errorf("维度 %d(%s) panic: %v", dim.ID, dim.Name, v)
	}
}

// SubmitTask 提交一次任务。
// payload 是完整的 addCircle 请求体（29 字段透传）。
func (c *Client) SubmitTask(ctx context.Context, token string, payload types.TaskSubmitPayload) (*types.TaskResult, error) {
	// 验证 payload
	if payload.CircleTaskID == 0 || payload.CircleTypeID == 0 {
		return nil, fmt.Errorf("%w: circleTaskId 和 circleTypeId 不能为空", ErrInvalidPayload)
	}

	resp, err := c.doBizAndDecode(ctx, token, "SubmitTask", "/api/studentCircleNew/addCircle", http.MethodPost, payload)
	if err != nil {
		// 保持原语义：业务错误返回 (result, error)，网络/解析错误返回 (nil, error)
		var bizErr *types.BusinessError
		if errors.As(err, &bizErr) {
			return &types.TaskResult{Code: bizErr.Code, Msg: bizErr.Msg}, err
		}
		return nil, err
	}

	return &types.TaskResult{
		Code: resp.Code,
		Msg:  types.DerefOr(resp.Msg, ""),
	}, nil
}

// GetDimensions 获取任务维度列表。
//
// SDK 高级用户使用，CLI 暂未暴露此命令。
func (c *Client) GetDimensions(ctx context.Context, token string) ([]types.Dimension, error) {
	return c.fetchDimensions(ctx, token, "GetDimensions")
}
