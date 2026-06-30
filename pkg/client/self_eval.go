package client

import (
	"context"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// SubmitSelfEvaluation 提交自我评价文本。
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error {
	_, err := c.doBizAndDecode(ctx, token, "SubmitSelfEvaluation", "/api/studentMoralEduNew/addSelfEvaluation",
		http.MethodPost, map[string]string{"studentComment": comment})
	return err
}

// QuerySelfEvaluation 查询自我评价状态 + 教师评语。
//
// 使用 doBizGetDecode 的 fallback 链（returnData → dataMap → dataList[0]），
// 替换原有的 selfEvalGet + tryDecodeFallback 模式。
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error) {
	v, err := doBizGetDecode[types.SelfEvalStatus](c, ctx, token, "QuerySelfEvaluation",
		"/api/studentMoralEduNew/querySelfEvaluation",
		types.DecodeReturnData[types.SelfEvalStatus],
		types.DecodeDataMap[types.SelfEvalStatus],
		func(resp types.UnifiedResponse) (*types.SelfEvalStatus, error) {
			if resp.DataList == nil {
				return nil, nil
			}
			statuses, err := types.DecodeDataList[types.SelfEvalStatus](resp)
			if err != nil {
				return nil, err
			}
			if len(statuses) == 0 {
				return nil, nil
			}
			return &statuses[0], nil
		},
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// QuerySelfGradEvaluation 查询毕业状态。
//
// SDK 高级用户使用，CLI 暂未暴露此命令。
// 使用 doBizGetDecode 的 fallback 链（returnData → dataMap），
// 替换原有的 selfEvalGet + tryDecodeFallback 模式。
func (c *Client) QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error) {
	v, err := doBizGetDecode[map[string]any](c, ctx, token, "QuerySelfGradEvaluation",
		"/api/studentMoralEduNew/querySelfGradEvaluation",
		types.DecodeReturnData[map[string]any],
		types.DecodeDataMap[map[string]any],
	)
	if err != nil {
		return nil, err
	}
	return v, nil
}
