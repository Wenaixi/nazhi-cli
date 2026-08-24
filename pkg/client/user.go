package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// GetMyInfo 获取完整的用户个人资料。
// 包含：姓名、性别、学号、学校、年级、班级、座号（seat）等。
//
// 错误契约：
//   - 网络/HTTP 失败 → 返回 (nil, fmt.Errorf("GetMyInfo 请求失败: %w", err))
//   - 业务 code≠1    → 返回 (nil, fmt.Errorf("获取用户信息业务错误: %w", errors.Join(ErrBusinessRejected, err)))
//   - returnData + dataMap 都为空（服务端成功响应但确实无用户数据）→ 返回 (nil, fmt.Errorf("%w: ...", ErrEmptyUserInfo))
//
// 调用方应使用 errors.Is 分支判定，**不要**用 `if info == nil { ... }` 兜底：
//   - `errors.Is(err, client.ErrEmptyUserInfo)`  → 业务成功但无数据，可走 status envelope
//   - `errors.Is(err, client.ErrBusinessRejected)` → 服务端主动拒绝（如 session 过期）
//   - 其他 err                                      → 真正的网络/HTTP 故障
//
// 业务成功但无数据时返回 ErrEmptyUserInfo 哨兵，cmd 层可据此走 status envelope。
func (c *Client) GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error) {
	// ActivateSession 若由步骤 4 完成激活会返回其获取的 UserInfo；
	// GetMyInfo 直接复用避免重复请求。session 已激活（fast path）时返回 nil,nil。
	info, err := c.ActivateSession(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("GetMyInfo 预热 session 失败: %w", err)
	}
	if info != nil {
		return info, nil
	}
	return c.getMyInfoRaw(ctx, token)
}

// getMyInfoRaw 是 GetMyInfo 的内部版本（不预热 session），供 ActivateSession
// 步骤 4 调用——避免外层持 sm.mu 时重入死锁。
//
// 注意：本方法不迁移到 doBizGetDecode，因为它需要自定义 Referer header (/modify)，
// 而 doBizGetDecode/doBizAndDecode 内部固定使用 bizHeaders()（Referer=/homepage）。
func (c *Client) getMyInfoRaw(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)
	// 固定 /modify 为 SDK 约定，非前端精确值，服务端不校验（前端实际为页面路径，服务端不校验 Referer）。
	headers["Referer"] = c.bizURL("/modify")

	bodyBytes, err := c.httpDo(ctx, http.MethodGet,
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
		return nil, fmt.Errorf("获取用户信息业务错误: %w", errors.Join(ErrBusinessRejected, err))
	}

	for _, dec := range []func() (*types.UserInfo, error){
		func() (*types.UserInfo, error) { return types.DecodeReturnData[types.UserInfo](resp) },
		func() (*types.UserInfo, error) { return types.DecodeDataMap[types.UserInfo](resp) },
	} {
		v, dErr := dec()
		if dErr == nil && v != nil {
			c.postProcessUserInfo(ctx, v)
			return v, nil
		}
		if dErr != nil {
			c.logDebug("GetMyInfo fallback: %v", dErr)
		}
	}
	return nil, fmt.Errorf("%w: returnData 和 dataMap 都为空", ErrEmptyUserInfo)
}

// postProcessUserInfo 对解析后的 UserInfo 做后处理。
// 包含：学校信息 SSO 降级、班级名年级前缀清理。
func (c *Client) postProcessUserInfo(ctx context.Context, v *types.UserInfo) {
	// 学校信息 SSO 降级：当 schoolId 或 schoolName 任一缺失时，通过 GetSchoolID 公开 API
	// 查询补全。GetSchoolID 无需 token，公开可用，条件放宽覆盖面。
	if v.StudentNumber != "" && (v.SchoolID == 0 || v.SchoolName == "") {
		if info, sErr := c.GetSchoolID(ctx, v.StudentNumber); sErr == nil {
			if v.SchoolID == 0 {
				if parsed, pErr := strconv.ParseInt(info.SchoolID, 10, 64); pErr == nil && parsed > 0 {
					v.SchoolID = parsed
				}
			}
			if v.SchoolName == "" && info.SchoolName != "" {
				v.SchoolName = info.SchoolName
			}
		} else {
			c.logDebug("GetMyInfo school fallback 失败: %v", sErr)
		}
	}

	// 前端 userBox/modifyBox/header 统一只移除首个“级”字，而不是按 GradeName 删除前缀。
	if v.ClassName != "" {
		v.ClassName = strings.Replace(v.ClassName, "级", "", 1)
	}
}
