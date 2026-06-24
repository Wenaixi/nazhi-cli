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

	// 步骤2/3 共享 getMenu 行为（仅 Referer 不同）→ 提取 doGetMenu helper。
	menuURL := c.bizURL("/api/studentInfo/getMenu")

	// 步骤2 响应体不参与兜底解析，但请求必须发出以满足 HAR 4 步契约。
	// helper 内部已 drain+close，丢弃返回的 body 即可。
	if _, err := c.doGetMenu(ctx, menuURL, headers, c.baseURL+"/homepage?token="+token, "步骤2"); err != nil {
		return nil, err
	}

	// 步骤3：GET /api/studentInfo/getMenu（Referer: /home）
	// body 需保留以供步骤 4 失败时兜底解析。
	step3Body, err := c.doGetMenu(ctx, menuURL, headers, c.baseURL+"/home", "步骤3")
	if err != nil {
		return nil, err
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
	if json.Unmarshal(step3Body, &unified) == nil && unified.Code == 1 {
		info, err := types.DecodeReturnData[types.UserInfo](unified)
		if err == nil && info != nil {
			info.Raw = parseRawData(*unified.ReturnData)
			return info, nil
		}
	}

	// 最坏情况：返回最少信息
	return &types.UserInfo{
		Raw: parseRawData(step3Body),
	}, nil
}

// doGetMenu 执行一次 getMenu 请求并返回响应体字节。
//
// helper 抽取动机：ActivateSession 步骤 2/3 几乎完全相同（同样的 URL、
// 同样的方法、差异仅在 Referer），inline 实现重复 ~14 行。统一在此处理
// 头复制、drain+close 资源回收，调用方只关心 referer 与错误标签。
//
// stepLabel 是用于错误信息的人类可读标签（如 "步骤2" / "步骤3"），调用方
// 需自行保证唯一性以便错误诊断。
func (c *Client) doGetMenu(ctx context.Context, menuURL string, baseHeaders map[string]string, referer, stepLabel string) ([]byte, error) {
	stepHeaders := copyMap(baseHeaders)
	stepHeaders["Referer"] = referer

	resp, err := c.doRequestWithResp(ctx, http.MethodGet, menuURL, nil, stepHeaders, "")
	if err != nil {
		return nil, fmt.Errorf("ActivateSession %s（getMenu）失败: %w", stepLabel, err)
	}
	defer func() {
		// 关键：先 drain body 再 close，让 net/http 把连接归还 keep-alive 池
		//（未 drain 的 body 在 Close 时强制关闭 TCP 连接，无法复用）
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ActivateSession %s 读取 getMenu 响应失败: %w", stepLabel, err)
	}
	return body, nil
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
