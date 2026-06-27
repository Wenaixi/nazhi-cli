package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// fetchTasksConcurrentLimit 是 FetchTasks 并发拉取维度的上限（F2 修复）。
//
// 设计权衡：业务系统实际维度数通常 ≤ 20，单次 FetchTasks 并发度受维度数封顶，
// 远低于 DoS 阈值。限制 = min(len(dimensions), 8) 平衡 wall time 与服务端压力：
//   - 8 路并发足够让 20 维度在 ~3 RTT 内完成（vs 串行 20 RTT）
//   - 不会因下游抖动放大熔断风险
//
// 如未来业务接口维度数 > 50，可考虑调到此常量或暴露为 Client 字段。
const fetchTasksConcurrentLimit = 8

// fetchDimensions 拉取任务维度列表（FetchTasks / GetDimensions 共用）。
// 内部包含 session 预热 + 4 段响应解析，错误信息前缀由 caller 决定。
func (c *Client) fetchDimensions(ctx context.Context, token string, errPrefix string) ([]types.Dimension, error) {
	if _, err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("%s 预热 session 失败: %w", errPrefix, err)
	}
	headers := c.bizHeaders(token)

	body, err := c.doRequest(ctx, http.MethodGet,
		c.bizURL("/api/studentCircleNew/getDimensions"),
		nil, headers, "",
	)
	if err != nil {
		return nil, fmt.Errorf("%s 请求失败: %w", errPrefix, err)
	}

	resp, err := types.DecodeResponse(body)
	if err != nil {
		return nil, fmt.Errorf("%s 响应解析失败: %w", errPrefix, err)
	}
	if err := types.CheckCode(resp); err != nil {
		// F-GroupD-E：与其他业务错误统一用 ErrBusinessRejected 包装。
		// 用 resp.Code/resp.Msg 直接拼字符串（与 SubmitTask 一致），
		// 不把 err 放 %w 位（否则 ErrBusinessRejected 不在链上）。
		// F19 (round-7) 重构：走 derefOr helper，与 auth.go:156/212 对齐。
		msg := derefOr(resp.Msg, "")
		return nil, fmt.Errorf("%w: %s 业务错误: code=%d msg=%s", ErrBusinessRejected, errPrefix, resp.Code, msg)
	}

	dimensions, err := types.DecodeDataList[types.Dimension](resp)
	if err != nil {
		return nil, fmt.Errorf("%s 维度列表解析失败: %w", errPrefix, err)
	}
	return dimensions, nil
}

// FetchTasks 拉取目标平台全部维度的任务列表。
// 内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。
//
// 并发拉取（F2 修复）：多个维度的 getCircleStatistics 通过 errgroup 并发执行，
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
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	// 每个 goroutine 写各自的本地切片，errgroup 不保护共享切片写入，
	// 所以最后在主线程用 mutex 串行合并，避免 race。
	var mu sync.Mutex
	var allTasks []types.Task
	var dimErrs []error // F-GroupD-F 修复：业务错误不再静默，累积而非 fail-fast
	for _, dim := range dimensions {
		// 跳过"全部"维度（id=0），它只是汇总
		if dim.ID == 0 {
			continue
		}
		dim := dim // 捕获循环变量
		// r9-D2 修复：循环变量捕获强约束。
		//
		// Go 1.22 之前每个 iteration 复用同一 dim 变量，
		// 必须显式 dim := dim 捕获当前值给 goroutine 使用。
		// Go 1.22+ 每个 iteration 自动生成新变量，此行变成无害冗余。
		// 本项目 go.mod 指定 Go 1.26.1，已具备新语义，但保留 dim := dim
		// 与显式注释作为防御——避免未来 refactor 把循环改为函数并意外丢捕获。
		// 维护者约束：删除此行前请确认循环变量语义与 goroutine 闭包兼容。
		g.Go(func() error {
			// F6-FETCHTASKS-CTX-CANCEL 修复：context 取消后直接 propagate，
			// 防止 cancel 被 dimErrs 吞掉后包装为 ErrBusinessRejected，
			// 调用方无法区分完整成功与 cancel 截断。
			if err := gctx.Err(); err != nil {
				return err
			}
			tasks, dimErr := c.fetchTasksForDimensionSafe(gctx, dim, headers)
			if dimErr != nil {
				mu.Lock()
				dimErrs = append(dimErrs, dimErr)
				mu.Unlock()
				// 不返回 error：保留"单维度失败不影响其他维度"语义，
				// 失败信息通过 dimErrs 聚合后整体 propagate。
				return nil
			}
			if tasks == nil {
				return nil
			}
			mu.Lock()
			allTasks = append(allTasks, tasks...)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		// r9-D1 修复（v0.3.5+）：errgroup 因 context 取消返回 error 时，
		// 若已有部分维度成功完成（allTasks 非空），应包装 ErrBusinessRejected。
		//
		// 动机：g.Go 闭包中 gctx.Err() 检查会在 ctx 取消后返回 DeadlineExceeded
		// 给 errgroup，导致 g.Wait() 返回 context error 并丢弃 allTasks。
		// 有 partial tasks 时包装 ErrBusinessRejected 让 cmd 层 envelope 可识别。
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if len(allTasks) > 0 {
				return allTasks, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
					ErrBusinessRejected, err)
			}
			return nil, err
		}
		return nil, fmt.Errorf("FetchTasks 并发拉取失败: %w", err)
	}

	// T1 round-5 修复：partial failures 聚合，区分全失败与部分失败。
	//
	// B11 修复：区分 context 取消错误与业务错误。
	// context.Canceled/DeadlineExceeded 不应包装为 ErrBusinessRejected，
	// 否则调用方通过 errors.Is(err, ErrBusinessRejected) 判定时会误判
	// 为业务拒绝，而非 context 截断，导致无法正确区分「完整成功」与
	// 「cancel 截断」两种语义。
	//
	// r9-D1 修复（v0.3.5+）：有 partial tasks 时仍在 ctx cancel 路径
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
	if len(dimErrs) > 0 {
		// 将 context 取消错误分离出来
		var bizErrs []error
		for _, de := range dimErrs {
			if errors.Is(de, context.Canceled) || errors.Is(de, context.DeadlineExceeded) {
				continue
			}
			bizErrs = append(bizErrs, de)
		}

		// 仅有 context 取消错误
		if len(bizErrs) == 0 {
			joined := errors.Join(dimErrs...)
			if len(allTasks) == 0 {
				// 无 partial tasks：裸返回，不包装 ErrBusinessRejected
				return nil, joined
			}
			// 有 partial tasks：包装 ErrBusinessRejected 让 cmd 层 envelope 识别
			return allTasks, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
				ErrBusinessRejected, joined)
		}

		// 有真正的业务错误，保持原 ErrBusinessRejected 包装语义
		joined := errors.Join(bizErrs...)
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
// 错误处理双路径（F-GroupD-F 修复）：
//   - HTTP / 网络错误（如连接超时、500）：走 logDebug + return (nil, nil)，
//     best-effort 模式不中断整体拉取（网络抖动不应当 fail-fast）
//   - 业务错误（code != 1）：return error，由 FetchTasks errgroup 聚合后
//     包装为 ErrBusinessRejected propagate，**不**静默吞咽
//
// 区分意义：网络抖动是「临时性、可重试」，业务错误是「服务端明确拒绝」，
// SDK 用户需要知道业务错误才能做精确处理（提示用户、报告 bug 等）。
// G2 (round-7) 修复：改用命名返回值 (tasks []types.Task, err error) 以便
// fetchTasksForDimensionSafe 的 defer recover 能通过闭包赋值捕获 panic。
// 非公有方法改变签名不破坏兼容性。
func (c *Client) fetchTasksForDimension(ctx context.Context, dim types.Dimension, headers map[string]string) (tasks []types.Task, err error) {
	// G1 round-6 修复：上下文取消（Canceled/DeadlineExceeded）直接 propagate，
	// 不吞掉走 best-effort——调用方需要知道 context 信号已触发，才能正确区分
	// 「真空数据」与「被取消」。
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// C13 说明：int64 参数纯数字，直接 strconv.FormatInt 拼接 URL 安全，
	// 无需 URL 编码（数字不包含特殊字符）。如需未来扩展为字符串参数，
	// 应改用 url.Values.Encode()。
	statURL := c.bizURL("/api/studentCircleNew/getCircleStatistics?dimensionId=" + strconv.FormatInt(dim.ID, 10))
	statBody, err := c.doRequest(ctx, http.MethodGet, statURL, nil, headers, "")
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err // 上下文取消应 propagate，不做 best-effort 吞没
		}
		c.logDebug("FetchTasks 维度 %d(%s) 请求失败: %v", dim.ID, dim.Name, err)
		return nil, nil // 网络错误走 best-effort
	}

	statResp, err := types.DecodeResponse(statBody)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 响应解析失败: %v", dim.ID, dim.Name, err)
		return nil, nil // 解析错误也走 best-effort（不归类为业务错误）
	}
	if statResp.Code != 1 {
		// F-GroupD-F：业务错误 propagate，不再静默。
		msg := derefOr(statResp.Msg, "")
		return nil, fmt.Errorf("维度 %d(%s) 业务错误: code=%d msg=%s", dim.ID, dim.Name, statResp.Code, msg)
	}

	tasks, err = types.DecodeDataList[types.Task](statResp)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 任务解析失败: %v", dim.ID, dim.Name, err)
		return nil, nil
	}

	for i := range tasks {
		tasks[i].DimensionName = dim.Name
	}
	return tasks, nil
}

// fetchTasksForDimensionSafe 是 fetchTasksForDimension 的 panic-safe 包装。
//
// G2 (round-7) 修复：errgroup.Go 闭包内无 panic recover 时，nil deref 或
// 第三方库 panic 会逃逸到 runtime → 进程崩溃 → g.Wait() 永不返回。
// 此 helper 在维度粒度捕获 panic，把它当业务错误记录到 dimErrs，
// 防止单个维度的 panic 影响其他维度的并发拉取。
//
// panic 信息：包含 dim.ID + dim.Name 便于排查（panic 路径无法
// 依赖 errgroup 自带的 nil-safe 包装，必须自己构建可读错误）。
func (c *Client) fetchTasksForDimensionSafe(ctx context.Context, dim types.Dimension, headers map[string]string) (tasks []types.Task, err error) {
	defer func() {
		if r := recover(); r != nil {
			tasks = nil
			err = fmt.Errorf("维度 %d(%s) panic: %v", dim.ID, dim.Name, r)
		}
	}()
	return c.fetchTasksForDimension(ctx, dim, headers)
}

// SubmitTask 提交一次任务。
// payload 是完整的 addCircle 请求体（29 字段透传）。
func (c *Client) SubmitTask(ctx context.Context, token string, payload types.TaskSubmitPayload) (*types.TaskResult, error) {
	headers := c.bizHeaders(token)

	// session 预热（HAR 强契约：4 步激活后再发 biz 请求，否则返回空数据）
	if _, err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("SubmitTask 预热 session 失败: %w", err)
	}

	// 验证 payload
	if payload.CircleTaskID == 0 || payload.CircleTypeID == 0 {
		return nil, fmt.Errorf("%w: circleTaskId 和 circleTypeId 不能为空", ErrInvalidPayload)
	}

	bodyBytes, err := c.doRequest(ctx, http.MethodPost,
		c.bizURL("/api/studentCircleNew/addCircle"),
		payload, headers, "",
	)
	if err != nil {
		return nil, fmt.Errorf("SubmitTask 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("SubmitTask 响应解析失败: %w", err)
	}

	result := &types.TaskResult{
		Code: resp.Code,
		Raw:  parseRawData(bodyBytes),
	}
	// C2 修复：用 derefOr 替代手动 nil 检查（与 auth.go:156/212 对齐）。
	result.Msg = derefOr(resp.Msg, "")

	if resp.Code != 1 {
		// F7 修复：用 ErrBusinessRejected 包装而非 ErrLoginRejected。
		// 业务错误（任务已提交/参数错/服务端 5xx）与登录状态无关，
		// 用户 errors.Is(err, ErrLoginRejected) 不应误判为需重新登录。
		return result, fmt.Errorf("%w: code=%d msg=%s", ErrBusinessRejected, resp.Code, result.Msg)
	}

	return result, nil
}

// GetDimensions 获取任务维度列表。
func (c *Client) GetDimensions(ctx context.Context, token string) ([]types.Dimension, error) {
	return c.fetchDimensions(ctx, token, "GetDimensions")
}

// GetCircleTypeByTaskID 确认任务类型信息。
//
// r9-D3 修复（2026-06-27 标记）：本方法在当前 SDK 中无业务调用方
// （仅 task_id_lookup_test.go 等测试使用），且维护负担较高——
// 每次响应结构变更需同步更新解析逻辑。
//
// 计划：保留半年观察期（至 2026-12-27），如仍无业务调用方则在下个
// major 版本（v0.4.0 或 v1.0.0）中删除。新业务应改用
// `SubmitTask` 的 payload 反查机制获取 circleTypeId。
//
// Deprecated: 此方法计划在下个 major 版本删除，新业务请勿依赖。
func (c *Client) GetCircleTypeByTaskID(ctx context.Context, token string, taskID int64) (*map[string]any, error) {
	if _, err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskID 预热 session 失败: %w", err)
	}
	headers := c.bizHeaders(token)

	// C13 说明：int64 参数纯数字，直接 strconv.FormatInt 拼接 URL 安全。
	url := c.bizURL("/api/studentCircleNew/getCircleTypeByTaskId?taskId=" + strconv.FormatInt(taskID, 10))
	bodyBytes, err := c.doRequest(ctx, http.MethodGet, url, nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskID 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskID 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		// F-GroupD-E：与其他业务错误统一用 ErrBusinessRejected 包装。
		// 用 resp.Code/resp.Msg 直接拼字符串（与 SubmitTask 一致），
		// 不把 err 放 %w 位（否则 ErrBusinessRejected 不在链上）。
		// F19 (round-7) 重构：走 derefOr helper，与 auth.go:156/212 对齐。
		msg := derefOr(resp.Msg, "")
		return nil, fmt.Errorf("%w: GetCircleTypeByTaskID 业务错误: code=%d msg=%s", ErrBusinessRejected, resp.Code, msg)
	}

	result, err := types.DecodeReturnData[map[string]any](resp)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskID returnData 解析失败: %w", err)
	}

	return result, nil
}
