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
		return nil, fmt.Errorf("%w: 获取用户信息业务错误: %v", ErrBusinessRejected, err)
	}

	// 两段 fallback（returnData → dataMap），用 tryDecodeFallback 消除重复
	v := tryDecodeFallback(c, "GetMyInfo",
		func() (*types.UserInfo, error) {
			u, err := types.DecodeReturnData[types.UserInfo](resp)
			if err == nil && u != nil {
				u.Raw = parseRawData(*resp.ReturnData)
			}
			return u, err
		},
		func() (*types.UserInfo, error) {
			u, err := types.DecodeDataMap[types.UserInfo](resp)
			if err == nil && u != nil {
				u.Raw = parseRawData(*resp.DataMap)
			}
			return u, err
		},
	)
	if v != nil {
		return v, nil
	}

	// F10 修复（round-7）：returnData + dataMap 都为 nil 时（业务成功响应
	// 但确实无用户数据），返回 ErrEmptyUserInfo 哨兵而非 (nil, nil)。
	//
	// 设计动机：
	//   - (nil, nil) 让 cmd 层只能裸输出 null，与 whoami 的 status envelope 不一致
	//   - 返回 ErrEmptyUserInfo 让 cmd 层用 errors.Is 分支统一走 status envelope
	//   - SDK 最佳努力契约保留（GetMyInfo 调用方通常吞错，但 err 提供语义信号）
	//
	// 与 ErrBusinessRejected 的语义边界：
	//   - ErrEmptyUserInfo: 服务端成功（code=1）但确实无数据，不是错误
	//   - ErrBusinessRejected: 服务端主动拒绝（code=0）
	return nil, fmt.Errorf("%w: returnData 和 dataMap 都为空", ErrEmptyUserInfo)
}
