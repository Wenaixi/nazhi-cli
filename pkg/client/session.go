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
// HAR 验证：必须先 GET / + GET /api/studentInfo/getMenu，否则后续接口返回空数据。
// 返回用户基本信息。
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)

	// 步骤1：GET /（首页）
	resp, err := c.doRequestWithResp(ctx, http.MethodGet, c.baseURL+"/", nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("ActivateSession 首页访问失败: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// 步骤2：GET /api/studentInfo/getMenu（激活 session）
	menuURL := c.bizURL("/api/studentInfo/getMenu")
	menuHeaders := copyMap(headers)
	menuHeaders["Referer"] = c.baseURL + "/home"

	menuResp, err := c.doRequestWithResp(ctx, http.MethodGet, menuURL, nil, menuHeaders, "")
	if err != nil {
		return nil, fmt.Errorf("ActivateSession getMenu 失败: %w", err)
	}
	defer menuResp.Body.Close()

	// 尝试从 getMenu 响应中解析用户信息
	bodyBytes, _ := io.ReadAll(menuResp.Body)
	var unified types.UnifiedResponse
	if json.Unmarshal(bodyBytes, &unified) == nil && unified.Code == 1 {
		userInfo, err := types.DecodeReturnData[types.UserInfo](unified)
		if err == nil && userInfo != nil {
			return userInfo, nil
		}
	}

	// 如果 getMenu 没有返回完整的 UserInfo，返回基础信息
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
