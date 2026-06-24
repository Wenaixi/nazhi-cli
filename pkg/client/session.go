package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ActivateSession 初始化目标平台业务 Session。
// HAR 验证（登录.har + 首页访问.har）：必须按以下 4 步顺序激活，否则后续接口返回空数据：
//  1. GET /（首页）
//  2. GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
//  3. GET /api/studentInfo/getMenu（Referer: /home）
//  4. GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
//
// 返回用户基本信息（含座号）。
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)

	// 步骤1：GET /（首页，建立业务域 session）
	if _, err := c.doBizGet(ctx, c.baseURL+"/", headers); err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤1（首页）失败: %w", err)
	}

	// 步骤2：GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
	menuURL := c.bizURL("/api/studentInfo/getMenu")
	step2Headers := copyMap(headers)
	step2Headers["Referer"] = c.baseURL + "/homepage?token=" + token

	if _, err := c.doBizGet(ctx, menuURL, step2Headers); err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤2（getMenu）失败: %w", err)
	}

	// 步骤3：GET /api/studentInfo/getMenu（Referer: /home）
	// 关键：保留 body 供步骤 4 兜底解析（getMyInfo 失败时使用 getMenu 响应中的 name）。
	step3Headers := copyMap(headers)
	step3Headers["Referer"] = c.baseURL + "/home"

	menuBytes, err := c.doBizGet(ctx, menuURL, step3Headers)
	if err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤3（getMenu）失败: %w", err)
	}

	// 步骤4：GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
	// 关键：用内部 getMyInfoRaw 而非公开 GetMyInfo，避免外层 sessionOnce.Do
	// 持锁时再次进入 sessionOnce.Do 死锁（reentrancy 限制）。
	userInfo, err := c.getMyInfoRaw(ctx, token)
	if err != nil {
		// 最佳努力：getMyInfo 失败不中断，仅 warn
		c.logDebug("ActivateSession 步骤4（getMyInfo）失败: %v", err)
	}

	if userInfo != nil && userInfo.Name != "" {
		return userInfo, nil
	}

	// 尝试从步骤3的 getMenu 响应中兜底解析
	var unified types.UnifiedResponse
	if json.Unmarshal(menuBytes, &unified) == nil && unified.Code == 1 {
		info, err := types.DecodeReturnData[types.UserInfo](unified)
		if err == nil && info != nil {
			info.Raw = parseRawData(*unified.ReturnData)
			return info, nil
		}
	}

	// 最坏情况：返回最少信息
	return &types.UserInfo{
		Raw: parseRawData(menuBytes),
	}, nil
}

// copyMap 复制 map[string]string。
func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// activateSessionIfNeeded 保证所有 biz 方法在第一次调用前完成
// 4 步 session 预热（HAR 验证的强契约），后续调用 token 相同则直接返回。
//
// token-aware 守卫：用 sessionToken + sessionMu 替代原 sync.Once，
// 解决 sync.Once 不感知 token 变更的问题——进程内 token 变化
// （如重新 Login）时重新执行 4 步激活，不会返回旧 session cookie。
func (c *Client) activateSessionIfNeeded(ctx context.Context, token string) error {
	c.sessionMu.Lock()
	if c.sessionToken == token {
		c.sessionMu.Unlock()
		return nil
	}
	c.sessionMu.Unlock()

	_, err := c.ActivateSession(ctx, token)
	if err == nil {
		c.sessionMu.Lock()
		c.sessionToken = token
		c.sessionMu.Unlock()
	}
	return err
}
