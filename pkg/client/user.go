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
		// 激活路径返回：ActivateSession 出口已做学校回退，直接复用。
		return info, nil
	}
	info2, err := c.getMyInfoRaw(ctx, token)
	if err != nil {
		return nil, err
	}
	// 本分支在 sm.mu 锁外（fast path 未命中且激活已完成的场景），可安全发网络回退。
	c.postProcessSchoolFallback(ctx, info2)
	c.sm.UpdateCachedUserInfo(info2)
	return info2, nil
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

	resp, err := decodeOrInvalidResponse("GetMyInfo", bodyBytes)
	if err != nil {
		return nil, err
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
			// P1-B：此处位于激活持锁路径内（getMyInfoRaw 被激活步骤 4 复用），
			// 只做纯 CPU 清理；学校 SSO 回退由 ActivateSession/GetMyInfo 出口在锁外补做。
			c.postProcessUserInfo(v)
			return v, nil
		}
		if dErr != nil {
			c.logDebug("GetMyInfo fallback: %v", dErr)
		}
	}
	return nil, fmt.Errorf("%w: returnData 和 dataMap 都为空", ErrEmptyUserInfo)
}

// postProcessUserInfo 对解析后的 UserInfo 做后处理（纯 CPU 段）。
// 仅包含班级名年级前缀清理；学校信息的 SSO 网络回退已拆分到
// postProcessSchoolFallback，必须在 sm.mu 锁外调用（P1-B）。
// 本方法在 getMyInfoRaw 解码成功点调用——该点位于激活持锁路径内，
// 只允许 O(1) 内存操作，禁止任何网络往返。
func (c *Client) postProcessUserInfo(v *types.UserInfo) {
	// 前端 userBox/modifyBox/header 统一只移除首个“级”字，而不是按 GradeName 删除前缀。
	if v.ClassName != "" {
		v.ClassName = strings.Replace(v.ClassName, "级", "", 1)
	}
}

// postProcessSchoolFallback 学校信息 SSO 回退补全（网络往返，仅限锁外调用）。
// 当 schoolId 或 schoolName 任一缺失时，通过 GetSchoolID 公开 API 查询补全；
// GetSchoolID 无需 token，公开可用。条件即幂等标记：补全完成后 SchoolID 与
// SchoolName 均非空，重复调用不会再发请求。
//
// P1-B：本函数曾内联在激活持锁路径中，一次 SSO 往返最坏把 sm.mu 锁窗口从
// 数百毫秒放大到 c.http.Timeout 秒级，违背 ActivateSession godoc 并发契约。
// 失败仅 logDebug 不致命（回退是尽力而为，服务端数据缺失由业务层裁决）。
func (c *Client) postProcessSchoolFallback(ctx context.Context, v *types.UserInfo) {
	if v.StudentNumber == "" || (v.SchoolID != 0 && v.SchoolName != "") {
		return
	}
	info, err := c.GetSchoolID(ctx, v.StudentNumber)
	if err != nil {
		c.logDebug("GetMyInfo school fallback 失败: %v", err)
		return
	}
	if v.SchoolID == 0 {
		if parsed, pErr := strconv.ParseInt(info.SchoolID, 10, 64); pErr == nil && parsed > 0 {
			v.SchoolID = parsed
		}
	}
	if v.SchoolName == "" && info.SchoolName != "" {
		v.SchoolName = info.SchoolName
	}
}
