package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// SubmitSelfEvaluation 提交自我评价文本。
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error {
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return fmt.Errorf("SubmitSelfEvaluation 预热 session 失败: %w", err)
	}
	headers := c.bizHeaders(token)

	bodyBytes, err := c.doRequest(ctx, http.MethodPost,
		c.bizURL("/api/studentMoralEduNew/addSelfEvaluation"),
		map[string]string{"studentComment": comment},
		headers, "",
	)
	if err != nil {
		return fmt.Errorf("SubmitSelfEvaluation 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return fmt.Errorf("SubmitSelfEvaluation 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		// F-GroupD-E：业务错误统一用 ErrBusinessRejected 包装（与 SubmitTask 对齐），
		// 让 SDK 用户能 errors.Is(err, ErrBusinessRejected) 精确判定，不会被误
		// 导为 ErrLoginRejected 而错误地走重新登录流程。
		// 不能用 err 作 %w（否则 ErrBusinessRejected 不在 err 链上，errors.Is 失败）。
		return fmt.Errorf("%w: 自我评价提交失败: %v", ErrBusinessRejected, err)
	}

	return nil
}

// QuerySelfEvaluation 查询自我评价状态 + 教师评语。
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error) {
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("QuerySelfEvaluation 预热 session 失败: %w", err)
	}
	headers := c.bizHeaders(token)

	bodyBytes, err := c.doRequest(ctx, http.MethodGet,
		c.bizURL("/api/studentMoralEduNew/querySelfEvaluation"),
		nil, headers, "",
	)
	if err != nil {
		return nil, fmt.Errorf("QuerySelfEvaluation 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("QuerySelfEvaluation 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		// F-GroupD-E：业务错误统一用 ErrBusinessRejected 包装。
		return nil, fmt.Errorf("%w: 查询自我评价失败: %v", ErrBusinessRejected, err)
	}

	// 尝试从 returnData 解析
	if resp.ReturnData != nil {
		status, err := types.DecodeReturnData[types.SelfEvalStatus](resp)
		if err == nil && status != nil {
			return status, nil
		}
		if err != nil {
			c.logDebug("QuerySelfEvaluation DecodeReturnData 失败: %v", err)
		}
	}

	// 尝试从 dataMap 解析
	if resp.DataMap != nil {
		status, err := types.DecodeDataMap[types.SelfEvalStatus](resp)
		if err == nil && status != nil {
			return status, nil
		}
		if err != nil {
			c.logDebug("QuerySelfEvaluation DecodeDataMap 失败: %v", err)
		}
	}

	// 尝试从 dataList 解析（可能只有一条记录）
	if resp.DataList != nil {
		statuses, err := types.DecodeDataList[types.SelfEvalStatus](resp)
		if err == nil && len(statuses) > 0 {
			return &statuses[0], nil
		}
		if err != nil {
			c.logDebug("QuerySelfEvaluation DecodeDataList 失败: %v", err)
		}
	}

	return nil, fmt.Errorf("QuerySelfEvaluation: 未找到评价记录")
}

// QuerySelfGradEvaluation 查询毕业状态。
func (c *Client) QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error) {
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("QuerySelfGradEvaluation 预热 session 失败: %w", err)
	}
	headers := c.bizHeaders(token)

	bodyBytes, err := c.doRequest(ctx, http.MethodGet,
		c.bizURL("/api/studentMoralEduNew/querySelfGradEvaluation"),
		nil, headers, "",
	)
	if err != nil {
		return nil, fmt.Errorf("QuerySelfGradEvaluation 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("QuerySelfGradEvaluation 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		// F-GroupD-E：业务错误统一用 ErrBusinessRejected 包装。
		return nil, fmt.Errorf("%w: 查询学期评价失败: %v", ErrBusinessRejected, err)
	}

	// 优先尝试 returnData
	if resp.ReturnData != nil {
		result, err := types.DecodeReturnData[map[string]any](resp)
		if err == nil && result != nil {
			return result, nil
		}
	}

	// 兜底尝试 dataMap
	if resp.DataMap != nil {
		result, err := types.DecodeDataMap[map[string]any](resp)
		if err == nil && result != nil {
			return result, nil
		}
	}

	// 所有路径都为空是合法的"无学期评价"，但有 body 解析不了就是 bug
	return nil, fmt.Errorf("QuerySelfGradEvaluation: 响应中既无 returnData 也无 dataMap（code=1 但无数据）")
}
