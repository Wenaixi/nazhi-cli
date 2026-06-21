package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// SubmitSelfEvaluation 提交自我评价文本。
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error {
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
		return fmt.Errorf("自我评价提交失败: %w", err)
	}

	return nil
}

// QuerySelfEvaluation 查询自我评价状态 + 教师评语。
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error) {
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
		return nil, fmt.Errorf("查询自我评价失败: %w", err)
	}

	// 尝试从 returnData 解析
	if resp.ReturnData != nil {
		status, err := types.DecodeReturnData[types.SelfEvalStatus](resp)
		if err == nil && status != nil {
			return status, nil
		}
	}

	// 尝试从 dataMap 解析
	if resp.DataMap != nil {
		status, err := types.DecodeDataMap[types.SelfEvalStatus](resp)
		if err == nil && status != nil {
			return status, nil
		}
	}

	// 尝试从 dataList 解析（可能只有一条记录）
	if resp.DataList != nil {
		statuses, err := types.DecodeDataList[types.SelfEvalStatus](resp)
		if err == nil && len(statuses) > 0 {
			return &statuses[0], nil
		}
	}

	return nil, fmt.Errorf("QuerySelfEvaluation: 未找到评价记录")
}

// QuerySelfGradEvaluation 查询毕业状态。
func (c *Client) QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error) {
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
		return nil, fmt.Errorf("查询毕业状态失败: %w", err)
	}

	result, err := types.DecodeReturnData[map[string]any](resp)
	if err != nil || result == nil {
		// 尝试 dataMap
		result, err = types.DecodeDataMap[map[string]any](resp)
		if err != nil {
			return nil, nil
		}
	}

	return result, nil
}
