package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// SubmitSelfEvaluation 提交自我评价文本。
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error {
	_, err := c.doBizAndDecode(ctx, token, "SubmitSelfEvaluation", "/api/studentMoralEduNew/addSelfEvaluation",
		http.MethodPost, map[string]string{"studentComment": comment})
	return err
}

// selfEvalGet 内部辅助，消除 QuerySelfEvaluation / QuerySelfGradEvaluation 中
// session 预热 -> bizHeaders -> doRequest -> DecodeResponse -> CheckCode 公共管道。
//
// 返回值：解码后的 UnifiedResponse（通过 CheckCode 确认 code=1），可直接供 tryDecodeFallback 使用。
//
// 实现：委托给提取的通用 helper doBizAndDecode。
func (c *Client) selfEvalGet(ctx context.Context, token string, path string, opName string) (*types.UnifiedResponse, error) {
	return c.doBizAndDecode(ctx, token, opName, path, http.MethodGet, nil)
}

// QuerySelfEvaluation 查询自我评价状态 + 教师评语。
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error) {
	resp, err := c.selfEvalGet(ctx, token, "/api/studentMoralEduNew/querySelfEvaluation", "QuerySelfEvaluation")
	if err != nil {
		return nil, err
	}

	// 三段 fallback（returnData → dataMap → dataList），全部走 tryDecodeFallback 统一 helper。
	// dataList 兜底也提取到 tryDecodeFallback 中。
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
