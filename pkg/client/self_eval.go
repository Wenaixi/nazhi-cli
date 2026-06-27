package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// SubmitSelfEvaluation 提交自我评价文本。
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error {
	if _, err := c.activateSessionIfNeeded(ctx, token); err != nil {
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
		// B14: errors.Join 同时支持 errors.Is(ErrBusinessRejected) 和
		// errors.As(*BusinessError)，避免 %v 断开链。
		return errors.Join(ErrBusinessRejected, fmt.Errorf("自我评价提交失败: %w", err))
	}

	return nil
}

// selfEvalGet 内部辅助，消除 QuerySelfEvaluation / QuerySelfGradEvaluation 中
// session 预热 -> bizHeaders -> doRequest -> DecodeResponse -> CheckCode 公共管道。
//
// 返回值：解码后的 UnifiedResponse（通过 CheckCode 确认 code=1），可直接供 tryDecodeFallback 使用。
func (c *Client) selfEvalGet(ctx context.Context, token string, path string, opName string) (*types.UnifiedResponse, error) {
	if _, err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("%s 预热 session 失败: %w", opName, err)
	}
	headers := c.bizHeaders(token)

	bodyBytes, err := c.doRequest(ctx, http.MethodGet,
		c.bizURL(path),
		nil, headers, "",
	)
	if err != nil {
		return nil, fmt.Errorf("%s 请求失败: %w", opName, err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("%s 响应解析失败: %w", opName, err)
	}

	if err := types.CheckCode(resp); err != nil {
		// B14: errors.Join 同时支持 errors.Is(ErrBusinessRejected) 和
		// errors.As(*BusinessError)。
		return nil, errors.Join(ErrBusinessRejected, fmt.Errorf("%s失败: %w", opName, err))
	}
	return &resp, nil
}

// QuerySelfEvaluation 查询自我评价状态 + 教师评语。
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error) {
	resp, err := c.selfEvalGet(ctx, token, "/api/studentMoralEduNew/querySelfEvaluation", "QuerySelfEvaluation")
	if err != nil {
		return nil, err
	}

	// 三段 fallback（returnData → dataMap → dataList），全部走 tryDecodeFallback 统一 helper。
	// F6 修复：dataList 兜底也提取到 tryDecodeFallback 中。
	v := tryDecodeFallback(c, "QuerySelfEvaluation",
		func() (*types.SelfEvalStatus, error) { return types.DecodeReturnData[types.SelfEvalStatus](*resp) },
		func() (*types.SelfEvalStatus, error) { return types.DecodeDataMap[types.SelfEvalStatus](*resp) },
		func() (*types.SelfEvalStatus, error) {
			if resp.DataList == nil {
				return nil, nil
			}
			statuses, err := types.DecodeDataList[types.SelfEvalStatus](*resp)
			if err != nil {
				return nil, err
			}
			if len(statuses) == 0 {
				return nil, nil
			}
			return &statuses[0], nil
		},
	)
	if v != nil {
		return v, nil
	}

	return nil, fmt.Errorf("QuerySelfEvaluation: 未找到评价记录")
}

// QuerySelfGradEvaluation 查询毕业状态。
func (c *Client) QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error) {
	resp2, err := c.selfEvalGet(ctx, token, "/api/studentMoralEduNew/querySelfGradEvaluation", "QuerySelfGradEvaluation")
	if err != nil {
		return nil, err
	}

	// 两段 fallback（returnData → dataMap），用 tryDecodeFallback 消除重复
	v := tryDecodeFallback(c, "QuerySelfGradEvaluation",
		func() (*map[string]any, error) { return types.DecodeReturnData[map[string]any](*resp2) },
		func() (*map[string]any, error) { return types.DecodeDataMap[map[string]any](*resp2) },
	)
	if v != nil {
		return v, nil
	}

	// 所有路径都为空是合法的"无学期评价"，但有 body 解析不了就是 bug
	return nil, fmt.Errorf("QuerySelfGradEvaluation: 响应中既无 returnData 也无 dataMap（code=1 但无数据）")
}
