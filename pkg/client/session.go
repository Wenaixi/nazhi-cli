package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// defaultSessionBackoff 是激活失败后禁止重试的默认时间窗口。
// 默认 5 秒：大部分瞬时故障（网络抖动、服务端短时 5xx）在 5 秒内
// 恢复的概率低，回退重试可有效抑制 thundering herd 放大效应。
const defaultSessionBackoff = 5 * time.Second

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
	defer drainAndClose(resp.Body)

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
// 并发语义（持锁单检）：
//   - 持锁后检查 sessionToken 是否匹配；相同则直接放行，零额外开销
//   - 不同则继续持锁，串行执行 4 步 ActivateSession
//   - 期间 sessionToken 仍为旧值，其他 goroutine 继续排队等待
//   - 4 步成功后才把 sessionToken 写为新 token，再放锁
//
// 注意：这是"持锁单检"模式（锁内检查），而非"double-checked locking"
// （锁外 fast path + 锁内重检）。因为本函数始终在锁内，fast path
// 会自然被 Lock() 串行化，外层预检不节省任何开销。
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

	// 持锁确认当前 token，避免重复激活（锁内单次检查，无外层 fast-path）。
	// 调用方无需先自行检查，本函数会在持有锁后判断。
	if c.sessionToken == token {
		return nil
	}

	// 激活失败 backoff 保护：上次失败后短时间内直接返回缓存错误，
	// 避免 N 个 goroutine 各自重试 4 步激活（thundering herd
	// 放大服务端压力：无 backoff 时 N=10 导致 10 组 × 4 步 = 40
	// 次 HTTP 请求，有 backoff 时只需 1 组即被缓存抑制）。
	backoff := c.sessionBackoff
	if backoff <= 0 {
		backoff = defaultSessionBackoff
	}
	if c.lastActivationErr != nil && time.Since(c.lastAttemptAt) < backoff {
		return fmt.Errorf("激活 session 失败（上次重试 %v 前）: %w",
			time.Since(c.lastAttemptAt), c.lastActivationErr)
	}

	// 持锁激活：4 步串行执行，写 cookie jar 互斥
	if _, err := c.ActivateSession(ctx, token); err != nil {
		c.lastActivationErr = err
		c.lastAttemptAt = time.Now()
		return err
	}
	c.sessionToken = token
	c.lastActivationErr = nil
	return nil
}
