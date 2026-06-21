package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetMyInfo 获取完整的用户个人资料。
// 包含：姓名、性别、学号、学校、年级、班级、座号（seat）等。
// 最佳努力设计：失败返回 nil，不中断主流程。
func (c *Client) GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)
	headers["Referer"] = c.baseURL + "/modify"

	bodyBytes, err := c.doRequest(ctx, http.MethodGet,
		c.bizURL("/api/studentInfo/getMyInfo"),
		nil, headers, "",
	)
	if err != nil {
		return nil, fmt.Errorf("GetMyInfo 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("GetMyInfo 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		// 最佳努力设计：失败返回 nil 不中断主流程
		return nil, nil
	}

	// 尝试从 returnData 解析
	if resp.ReturnData != nil {
		userInfo, err := types.DecodeReturnData[types.UserInfo](resp)
		if err == nil && userInfo != nil {
			// 保留原始数据
			userInfo.Raw = parseRawData(*resp.ReturnData)
			return userInfo, nil
		}
	}

	// 尝试从 dataMap 解析
	if resp.DataMap != nil {
		userInfo, err := types.DecodeDataMap[types.UserInfo](resp)
		if err == nil && userInfo != nil {
			userInfo.Raw = parseRawData(*resp.DataMap)
			return userInfo, nil
		}
	}

	return nil, nil
}

// extractSeatFromMyInfo 从 getMyInfo 的原始响应中提取座号（seat）。
// getMyInfo 响应格式: {"code":1, "returnData": {"studentId":..., "seat":...}}
// seat 字段可能嵌套在 extra_json 或 profile 中。
func extractSeatFromMyInfo(raw map[string]any) int {
	if raw == nil {
		return 0
	}

	// 直接顶层 seat
	if seat, ok := raw["seat"].(float64); ok {
		return int(seat)
	}

	// 嵌套在 extra_json.profile.seat
	if extra, ok := raw["extra_json"].(map[string]any); ok {
		if profile, ok := extra["profile"].(map[string]any); ok {
			if seat, ok := profile["seat"].(float64); ok {
				return int(seat)
			}
		}
	}

	return 0
}

