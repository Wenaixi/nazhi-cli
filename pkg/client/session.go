package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
	resp, err := c.doRequestWithResp(ctx, http.MethodGet, c.baseURL+"/", nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤1（首页）失败: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	// 步骤2：GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
	menuURL := c.bizURL("/api/studentInfo/getMenu")
	step2Headers := copyMap(headers)
	step2Headers["Referer"] = c.baseURL + "/homepage?token=" + token

	step2Resp, err := c.doRequestWithResp(ctx, http.MethodGet, menuURL, nil, step2Headers, "")
	if err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤2（getMenu）失败: %w", err)
	}
	_, _ = io.Copy(io.Discard, step2Resp.Body)
	step2Resp.Body.Close()

	// 步骤3：GET /api/studentInfo/getMenu（Referer: /home）
	step3Headers := copyMap(headers)
	step3Headers["Referer"] = c.baseURL + "/home"

	menuResp, err := c.doRequestWithResp(ctx, http.MethodGet, menuURL, nil, step3Headers, "")
	if err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤3（getMenu）失败: %w", err)
	}
	defer menuResp.Body.Close()

	// 步骤4：GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
	userInfo, err := c.GetMyInfo(ctx, token)
	if err != nil {
		// 最佳努力：getMyInfo 失败不中断，仅 warn
		c.logDebug("ActivateSession 步骤4（getMyInfo）失败: %v", err)
	}

	if userInfo != nil && userInfo.Name != "" {
		return userInfo, nil
	}

	// 尝试从步骤3的 getMenu 响应中兜底解析
	bodyBytes, _ := io.ReadAll(menuResp.Body)
	var unified types.UnifiedResponse
	if json.Unmarshal(bodyBytes, &unified) == nil && unified.Code == 1 {
		info, err := types.DecodeReturnData[types.UserInfo](unified)
		if err == nil && info != nil {
			info.Raw = parseRawData(*unified.ReturnData)
			return info, nil
		}
	}

	// 最坏情况：返回最少信息
	return &types.UserInfo{
		Raw: parseRawData(bodyBytes),
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
