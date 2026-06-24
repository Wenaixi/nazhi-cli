package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ActivateSession 初始化目标平台业务 Session。
// HAR 验证（登录.har + 首页访问.har）：必须按以下 4 步顺序激活，否则后续接口返回空数据：
//  1. GET /（首页）
//  2. GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
//  3. GET /api/studentInfo/getMenu（Referer: /home）
//  4. GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
//
// 返回用户基本信息（含座号）。4 步任一失败立即 propagate error：
// 步骤 4（getMyInfo）是 4 步契约的一部分，失败不再走步骤 3 兜底掩盖
// （F10：曾 logDebug + 兜底解析，导致 getMyInfo 服务降级被静默吞掉，
// 后续业务接口返回空数据难以排查）。
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)

	// 步骤1：GET /（首页，建立业务域 session）
	if _, err := c.doBizGet(ctx, c.baseURL+"/", headers); err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤1（首页）失败: %w", err)
	}

	// 步骤2/3 共享 getMenu 行为（仅 Referer 不同）→ 提取 doGetMenu helper。
	menuURL := c.bizURL("/api/studentInfo/getMenu")

	// 步骤2 响应体不参与兜底解析，但请求必须发出以满足 HAR 4 步契约。
	// helper 内部已 drain+close，丢弃返回的 body 即可。
	// F1 修复：token 走 url.Values 编码，避免 & / = / 空格等字符破坏
	// Referer URL 结构（Referer 头会被浏览器/代理/服务端日志记录）。
	step2Referer := c.baseURL + "/homepage?" + url.Values{"token": {token}}.Encode()
	if _, err := c.doGetMenu(ctx, menuURL, headers, step2Referer, "步骤2"); err != nil {
		return nil, err
	}

	// 步骤3：GET /api/studentInfo/getMenu（Referer: /home）
	if _, err := c.doGetMenu(ctx, menuURL, headers, c.baseURL+"/home", "步骤3"); err != nil {
		return nil, err
	}

	// 步骤4：GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
	// 关键：用内部 getMyInfoRaw 而非公开 GetMyInfo，避免外层 sessionOnce.Do
	// 持锁时再次进入 sessionOnce.Do 死锁（reentrancy 限制）。
	// 失败 propagate：步骤 4 是 4 步 HAR 契约的一部分。
	return c.getMyInfoRaw(ctx, token)
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
// 并发语义（double-checked locking 完整模式）：
//   - 首次进入持锁检查 sessionToken；相同则直接放行，零开销
//   - 不同则继续持锁，串行执行 4 步 ActivateSession
//   - 期间 sessionToken 仍为旧值，其他 goroutine 继续排队等待
//   - 4 步成功后才把 sessionToken 写为新 token，再放锁
//
// 为何必须持锁激活：4 步会写入共享 c.http.Jar cookie jar，并发激活
// 会导致 cookie 状态机污染（thundering herd + 脏 cookie）。
// 持锁时间 ≈ 4 步网络 RTT（200-500ms），可接受。
//
// 与 sync.Once 的区别：sync.Once.Do(f) 保证 f 进程内只执行一次；本
// 实现感知 token 变化——同一 Client 上 token 变更（如重新 Login）时
// 会重新执行 4 步激活，确保 cookie jar 与当前 token 一致。
func (c *Client) activateSessionIfNeeded(ctx context.Context, token string) error {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()

	// 双检：持锁状态下再次确认 token，避免重复激活
	if c.sessionToken == token {
		return nil
	}

	// 持锁激活：4 步串行执行，写 cookie jar 互斥
	if _, err := c.ActivateSession(ctx, token); err != nil {
		return err
	}
	c.sessionToken = token
	return nil
}
