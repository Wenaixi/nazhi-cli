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
	if err := c.activateSessionIfNeeded(ctx, token); err != nil {
		return nil, fmt.Errorf("GetMyInfo 预热 session 失败: %w", err)
	}
	return c.getMyInfoRaw(ctx, token)
}

// getMyInfoRaw 是 GetMyInfo 的内部版本（不预热 session），供 ActivateSession
// 步骤 4 调用——避免外层 sessionOnce.Do 持锁时再次进入 sessionOnce.Do 死锁。
// 公开 SDK 用户请使用 GetMyInfo。
func (c *Client) getMyInfoRaw(ctx context.Context, token string) (*types.UserInfo, error) {
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
		return nil, fmt.Errorf("获取用户信息业务错误: %w", err)
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
