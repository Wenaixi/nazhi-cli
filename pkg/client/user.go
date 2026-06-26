package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetMyInfo 获取完整的用户个人资料。
// 包含：姓名、性别、学号、学校、年级、班级、座号（seat）等。
//
// 错误语义（r9-D9 修复契约）：
//   - 服务端业务拒绝（code != 1）→ 返回 (nil, errors.Join(ErrBusinessRejected, ...))
//   - 网络/解析失败 → 返回 (nil, ErrNetwork / 包装 err)
//   - 业务成功但 returnData + dataMap 都为空 → 返回 (nil, ErrEmptyUserInfo)
//
// 调用方应使用 errors.Is 判定哨兵错误，**不要**依赖 if info == nil 判定失败：
//
//	info, err := c.GetMyInfo(ctx, token)
//	if errors.Is(err, client.ErrEmptyUserInfo) {
//	    // 业务成功但无数据（可能是首次登录未填资料）
//	} else if err != nil {
//	    // 真错误
//	}
//	if info != nil { /* 使用数据 */ }
//
// 与 round-7 F10 修复后契约保持一致：旧版本「失败返回 nil 不中断主流程」
// 已被替换为「返回 (nil, ErrEmptyUserInfo) 供调用方精确判定」。
func (c *Client) GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error) {
	// B10 修复：activateSessionIfNeeded 返回步骤 4 获取的 UserInfo（若激活由
	// 步骤 4 完成），GetMyInfo 直接复用，避免重复的 getMyInfoRaw HTTP 请求。
	// session 已激活（fast path）时返回 nil,nil。
	info, err := c.activateSessionIfNeeded(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("GetMyInfo 预热 session 失败: %w", err)
	}
	if info != nil {
		// 激活步骤 4 已拿到数据，直接返回，无需额外 HTTP 请求
		return info, nil
	}
	return c.getMyInfoRaw(ctx, token)
}

// getMyInfoRaw 是 GetMyInfo 的内部版本（不预热 session），供 ActivateSession
// 步骤 4 调用——避免外层 sessionOnce.Do 持锁时再次进入 sessionOnce.Do 死锁。
// 公开 SDK 用户请使用 GetMyInfo。
func (c *Client) getMyInfoRaw(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)
	// r9-D8 修复：Referer 走 c.bizURL() helper，与其他业务接口对称
	// （避免 baseURL 拼接分散在多处，未来 baseURL 变更只需改 helper 一处）
	headers["Referer"] = c.bizURL("/modify")

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
		return nil, fmt.Errorf("获取用户信息业务错误: %v", errors.Join(ErrBusinessRejected, err))
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
