package client

import (
	"context"
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
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
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
		return nil, fmt.Errorf("%s 业务错误: %w", errPrefix, err)
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
	for _, dim := range dimensions {
		// 跳过"全部"维度（id=0），它只是汇总
		if dim.ID == 0 {
			continue
		}
		dim := dim // 捕获循环变量
		g.Go(func() error {
			tasks := c.fetchTasksForDimension(gctx, dim, headers)
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
		return nil, fmt.Errorf("FetchTasks 并发拉取失败: %w", err)
	}

	return allTasks, nil
}

// fetchTasksForDimension 拉取单个维度的任务列表并注入维度名称。
// 任何错误都通过 logDebug 记录后返回 nil（FetchTasks 整体不中断）。
func (c *Client) fetchTasksForDimension(ctx context.Context, dim types.Dimension, headers map[string]string) []types.Task {
	statURL := c.bizURL("/api/studentCircleNew/getCircleStatistics?dimensionId=" + strconv.FormatInt(dim.ID, 10))
	statBody, err := c.doRequest(ctx, http.MethodGet, statURL, nil, headers, "")
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 请求失败: %v", dim.ID, dim.Name, err)
		return nil
	}

	statResp, err := types.DecodeResponse(statBody)
	if err != nil || statResp.Code != 1 {
		c.logDebug("FetchTasks 维度 %d(%s) 响应异常: parseErr=%v code=%d", dim.ID, dim.Name, err, statResp.Code)
		return nil
	}

	tasks, err := types.DecodeDataList[types.Task](statResp)
	if err != nil {
		c.logDebug("FetchTasks 维度 %d(%s) 任务解析失败: %v", dim.ID, dim.Name, err)
		return nil
	}

	for i := range tasks {
		tasks[i].DimensionName = dim.Name
	}
	return tasks
}

// SubmitTask 提交一次任务。
// payload 是完整的 addCircle 请求体（29 字段透传）。
func (c *Client) SubmitTask(ctx context.Context, token string, payload types.TaskSubmitPayload) (*types.TaskResult, error) {
	headers := c.bizHeaders(token)

	// session 预热（HAR 强契约：4 步激活后再发 biz 请求，否则返回空数据）
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
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
	if resp.Msg != nil {
		result.Msg = *resp.Msg
	}

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

// GetCircleTypeByTaskId 确认任务类型信息。
func (c *Client) GetCircleTypeByTaskId(ctx context.Context, token string, taskID int64) (*map[string]any, error) {
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskId 预热 session 失败: %w", err)
	}
	headers := c.bizHeaders(token)

	url := c.bizURL("/api/studentCircleNew/getCircleTypeByTaskId?taskId=" + strconv.FormatInt(taskID, 10))
	bodyBytes, err := c.doRequest(ctx, http.MethodGet, url, nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskId 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskId 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskId 业务错误: %w", err)
	}

	result, err := types.DecodeReturnData[map[string]any](resp)
	if err != nil {
		return nil, fmt.Errorf("GetCircleTypeByTaskId returnData 解析失败: %w", err)
	}

	return result, nil
}
