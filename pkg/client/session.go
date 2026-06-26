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
//
// 并发安全：公开方法持 sessionMu 锁后调用 activateSessionLocked（不持锁），
// 防止外部调用与内部 activateSessionIfNeeded 并发执行 4 步激活污染 cookie jar
// （G8：原实现公开方法自身无锁，外部直接调 + 内部持锁调会并发写 cookie jar）。
//
// Backoff 缓存：失败时同步更新 c.lastActivationErr / c.lastAttemptAt /
// c.lastFailedToken。CLI 路径（直接调 ActivateSession）与业务方法路径
// （通过 activateSessionIfNeeded 间接调）共享同一份 backoff 缓存，
// 同 token 在窗口内的重复调用会被抑制。
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	return c.activateWithBackoffCheck(ctx, token)
}

// activateWithBackoffCheck 是激活的统一入口（持锁状态下）。
//
// 调用方契约：必须持 c.sessionMu 锁。本函数负责：
//  1. backoff 检查（同 token 在窗口内 → 返回 ErrSessionBackoff）
//  2. 4 步激活（持锁写 cookie jar）
//  3. 失败/成功时同步更新 lastActivationErr / lastAttemptAt / lastFailedToken / sessionToken
//
// F15 修复（round-7）：backoff 缓存键必须包含 token 维度。
// 切换 token 时（如 token-A 过期换 token-B），新 token 不应被旧 token
// 的失败缓存抑制 — 否则会返回 stale error 而不实际尝试新 token 激活。
func (c *Client) activateWithBackoffCheck(ctx context.Context, token string) (*types.UserInfo, error) {
	// backoff 检查：上次失败且同 token 在窗口内 → 抑制
	backoff := c.sessionBackoff
	if backoff <= 0 {
		backoff = defaultSessionBackoff
	}
	if c.lastActivationErr != nil &&
		c.lastFailedToken == token &&
		time.Since(c.lastAttemptAt) < backoff {
		// 返回包装 ErrSessionBackoff 的错误，SDK 用户可通过
		// errors.Is(err, ErrSessionBackoff) 识别「在冷却窗口内被抑制」。
		return nil, fmt.Errorf("%w: 上次 token %q 激活失败重试 %v 前，请稍后重试或换 token: %v",
			ErrSessionBackoff, token, time.Since(c.lastAttemptAt), c.lastActivationErr)
	}

	// 持锁激活：4 步串行执行，写 cookie jar 互斥
	info, err := c.activateSessionLocked(ctx, token)
	if err != nil {
		c.lastActivationErr = err
		c.lastAttemptAt = time.Now()
		c.lastFailedToken = token
		c.cachedUserInfo = nil
		return nil, err
	}
	c.sessionToken = token
	c.lastActivationErr = nil
	c.lastFailedToken = ""
	// B10 修复：缓存步骤 4 的 UserInfo，供 GetMyInfo 复用
	if info != nil {
		c.cachedUserInfo = info
	}
	return info, nil
}

// activateSessionLocked 是 ActivateSession 的内部 4 步实现，**调用方必须持 sessionMu 锁**。
//
// 持锁契约：调用方负责保证 c.sessionMu 已 Lock；本函数不重复 Lock 避免死锁。
// G8 修复（round-7）：拆出本 unexported 函数让 activateSessionIfNeeded 在持锁
// 状态下直接调用，避免 sync.Mutex 不可重入导致的死锁。
//
// 注意：本函数不写 lastActivationErr / lastFailedToken / sessionToken，
// 这些字段由 activateWithBackoffCheck 统一管理（避免分散到多处导致不一致）。
func (c *Client) activateSessionLocked(ctx context.Context, token string) (*types.UserInfo, error) {
	headers := c.bizHeaders(token)

	// 步骤1：GET /（首页，建立业务域 session）
	if _, err := c.doBizGet(ctx, c.bizURL("/"), headers); err != nil {
		return nil, fmt.Errorf("ActivateSession 步骤1（首页）失败: %w", err)
	}

	// 步骤2/3 共享 getMenu 行为（仅 Referer 不同）→ 提取 doGetMenu helper。
	menuURL := c.bizURL("/api/studentInfo/getMenu")

	// 步骤2 响应体不参与兜底解析，但请求必须发出以满足 HAR 4 步契约。
	// helper 内部已 drain+close，丢弃返回的 body 即可。
	// F1 修复：token 走 url.Values 编码，避免 & / = / 空格等字符破坏
	// Referer URL 结构（Referer 头会被浏览器/代理/服务端日志记录）。
	step2Referer := c.bizURL("/homepage") + "?" + url.Values{"token": {token}}.Encode()
	if _, err := c.doGetMenu(ctx, menuURL, headers, step2Referer, "步骤2"); err != nil {
		return nil, err
	}

	// 步骤3：GET /api/studentInfo/getMenu（Referer: /home）
	if _, err := c.doGetMenu(ctx, menuURL, headers, c.bizURL("/home"), "步骤3"); err != nil {
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
//
// G8 + F15 修复（round-7）：统一走 activateWithBackoffCheck（持锁版本），
// 共享 backoff 缓存与 sessionToken 更新逻辑，避免重复实现导致不一致。
func (c *Client) activateSessionIfNeeded(ctx context.Context, token string) (*types.UserInfo, error) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()

	// fast path：sessionToken 已匹配 → 零额外开销
	if c.sessionToken == token {
		// B10 修复：返回缓存的 UserInfo（步骤 4 已获取），避免 GetMyInfo 额外请求。
		return c.cachedUserInfo, nil
	}

	// 完整路径：backoff 检查 + 4 步激活 + 缓存更新（全部持锁）
	info, err := c.activateWithBackoffCheck(ctx, token)
	if err != nil {
		return nil, err
	}
	// B10 修复：返回步骤 4 获取的 UserInfo，供 GetMyInfo 复用，避免冗余 HTTP 请求。
	return info, nil
}
