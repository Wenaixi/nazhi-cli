package client

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

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
// 并发拉取：多个维度的 getCircleStatistics 通过 goroutine 并发执行，
// 维度 wall time 从 N × RTT 降到 ≈ RTT（受服务端并发上限约束）。
//
// 并发上限说明（review-tdd F12）：当前对每个 dimension 启一个 goroutine，
// 无 semaphore / worker pool 限制。业务系统实际维度数通常 ≤ 20，
// 单次 FetchTasks 并发度受维度数封顶，远低于 DoS 阈值。
// 如未来接入会返回 > 50 维度的业务接口，需引入 semaphore
// （如 golang.org/x/sync/semaphore，限制并发 = min(len(dimensions), 8)）。
//
// 单个维度失败时通过 c.logDebug() 记录（不会中断整体拉取），
// 调用方可通过 client.WithLogger() 注入自定义 logger 捕获详细错误。
func (c *Client) FetchTasks(ctx context.Context, token string) ([]types.Task, error) {
	dimensions, err := c.fetchDimensions(ctx, token, "FetchTasks getDimensions")
	if err != nil {
		return nil, err
	}

	headers := c.bizHeaders(token)

	// 3. 并发遍历每个维度获取任务统计。
	// 用 WaitGroup + 收集器；每个 goroutine 独立处理单维度错误（logDebug 记录后跳过）。
	results := make(chan []types.Task, len(dimensions))
	var wg sync.WaitGroup
	for _, dim := range dimensions {
		// 跳过"全部"维度（id=0），它只是汇总
		if dim.ID == 0 {
			continue
		}
		dim := dim // 捕获循环变量
		wg.Add(1)
		go func() {
			defer wg.Done()
			tasks := c.fetchTasksForDimension(ctx, dim, headers)
			if tasks != nil {
				results <- tasks
			}
		}()
	}
	wg.Wait()
	close(results)

	// 4. 聚合结果（顺序由 channel 决定，不保证业务顺序；保留原行为）。
	var allTasks []types.Task
	for tasks := range results {
		allTasks = append(allTasks, tasks...)
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
		return result, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, resp.Code, result.Msg)
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
