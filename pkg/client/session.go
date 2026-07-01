package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// defaultSessionBackoff 是激活失败后禁止重试的默认时间窗口。默认 5 秒。
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
// （曾 logDebug + 兜底解析，导致 getMyInfo 服务降级被静默吞掉，
// 后续业务接口返回空数据难以排查）。
//
// 内部实现：委托给 sm.Activate，由 sessionManager 负责 DCL fast path、
// backoff 检查、持锁 4 步激活和状态记录。本方法不额外持锁。
//
// B4 外部并发契约：
//   - 本函数委托给 sm.Activate，后者在 sm.mu 持锁状态下执行 4 步网络请求
//     （200-500ms），持有锁期间不会回调外层或锁住其他互斥资源。
//   - 外部调用方应**避免**在本函数持锁路径内嵌套其他锁，
//     否则可能引发 ABBA 死锁（如 errgroup.Go 中先持锁 A 再调本函数，
//     本函数持 sm.mu 时反调锁 A）。
//   - 外部使用模式：直接 goroutine 并发调本函数是安全的——sm.mu
//     只会序列化 4 步激活，不会让其他 goroutine 饿死（约 200-500ms 内释放）。
//   - 如果需要在锁内调本函数（如 sync.Mutex 临界区），需确保外层锁
//     的获取/释放顺序一致，不会形成循环等待。
//
// 并发安全：本方法委托给 sm.Activate（内部持锁 DCL），
//
// Backoff 缓存：失败时通过 sm.RecordFailure 更新 lastErr / lastAttempt /
// lastFailedToken。CLI 路径（直接调 ActivateSession）与业务方法路径
// （通过 ActivateSession 间接调）共享同一份 backoff 缓存，
// 同 token 在窗口内的重复调用会被抑制。
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error) {
	return c.sm.Activate(ctx, token, c.activateSessionLocked)
}

// activateSessionLocked 是 ActivateSession 的内部 4 步实现，
// **调用方必须持 sm.mu 锁**。
//
// 持锁契约：sm.Activate 负责保证 sm.mu 已 Lock；本函数不重复 Lock 避免死锁。
// 拆出本 unexported 函数让内部方法在持锁状态下直接调用，避免 sync.Mutex
// 不可重入导致的死锁。
//
// 注意：本函数不写 lastErr / lastFailedToken / sessionToken / cachedUserInfo，
// 这些字段由 sm.Activate 统一管理（通过内部的 backoff 检查 + 4 步激活 +
// RecordFailure/RecordSuccess），避免分散到多处导致不一致。
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
	// token 走 url.Values 编码，避免 & / = / 空格等字符破坏
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
	// 关键：用内部 getMyInfoRaw 而非公开 GetMyInfo，避免 sm.mu（sync.Mutex）
	// 持锁时再次进入死锁（不可重入限制）。
	// 失败 propagate：步骤 4 是 4 步 HAR 契约的一部分。
	return c.getMyInfoRaw(ctx, token)
}

// doGetMenu 执行一次 getMenu 请求并返回响应体字节。
//
// helper 抽取动机：ActivateSession 步骤 2/3 几乎完全相同（同样的 URL、
// 同样的方法、差异仅在 Referer），inline 实现重复 ~14 行。统一在此处理
// 头复制、drain+close 资源回收，调用方只关心 referer 与错误标签。
//
// 注意：baseHeaders 不会被修改（直接覆盖 Referer，不 clone ——
// bizHeaders 每次返回新 map，无需 maps.Clone）。
//
// stepLabel 是用于错误信息的人类可读标签（如 "步骤2" / "步骤3"），调用方
// 需自行保证唯一性以便错误诊断。
func (c *Client) doGetMenu(ctx context.Context, menuURL string, baseHeaders map[string]string, referer, stepLabel string) ([]byte, error) {
	baseHeaders["Referer"] = referer

	resp, err := c.rawDoWithResp(ctx, http.MethodGet, menuURL, nil, baseHeaders, "")
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

// ─── sessionManager: 业务 API session 激活状态机 ───
//
// 职责范围：
//   - 4 步 HAR 激活流程（ActivateSession）
//   - backoff 缓存（失败后冷却抑制 thundering herd）
//   - double-checked locking fast path（sessionToken + cachedUserInfo）
//   - 内部 getMyInfo 缓存（GetMyInfo 复用步骤 4 数据）
//
// 并发安全：
//   - mu 保护所有状态变更（4 步激活写 cookie jar）
//   - token 通过 atomic.Value 读外写入（fast path 无锁读）
//   - cachedUserInfo 在 mu 临界区内写入
//
// 与 Client 的关系：
//   - Client 持有 *sessionManager，所有 session 相关调用委托给它
//   - isBackoffHit 等纯逻辑方法无锁，供外部测试时直接调用

// sessionManager 管理业务 API session 的激活状态机。
type sessionManager struct {
	mu      sync.Mutex
	token   atomic.Value // 存储 string，写路径在 mu 持锁状态下完成
	backoff time.Duration

	lastErr         error
	lastAttempt     time.Time
	lastFailedToken string
	cachedUserInfo  *types.UserInfo // DCL fast path 缓存。CLI 单进程命中一次，
	// SDK 多 goroutine 并发 FetchTasks 可复用步骤 4 数据。
}

// isBackoffHit 检查给定 token 是否在 backoff 冷却窗口内。
// 纯逻辑方法，不持锁，适合单元测试直接调用。
func (sm *sessionManager) isBackoffHit(token string) bool {
	if sm.lastErr == nil || sm.lastFailedToken != token {
		return false
	}
	backoff := sm.backoff
	if backoff <= 0 {
		backoff = defaultSessionBackoff
	}
	return time.Since(sm.lastAttempt) < backoff
}

// LoadToken 原子读当前 session token（fast path 用）。
// atomic.Value 存储类型必须是 string，type assertion 失败 panic 暴露编程错误。
func (sm *sessionManager) LoadToken() string {
	v := sm.token.Load()
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		panic(fmt.Sprintf("sessionManager: token 存储类型异常，期望 string 实际 %T", v))
	}
	return s
}

// clearBackoff 清除 backoff 状态（lastErr + lastFailedToken）。
// 调用方须持 sm.mu。
func (sm *sessionManager) clearBackoff() {
	sm.lastErr = nil
	sm.lastFailedToken = ""
}

// StoreToken 持锁写 token，并清除 backoff 状态。
// 调用方须持 sm.mu。
func (sm *sessionManager) StoreToken(token string) {
	sm.token.Store(token)
	sm.clearBackoff()
}

// RecordFailure 持锁记录激活失败，按 token 匹配决定是否清缓存。
// 调用方须持 sm.mu。
func (sm *sessionManager) RecordFailure(token string, err error) {
	sm.lastErr = err
	sm.lastAttempt = time.Now()
	sm.lastFailedToken = token
	// 只有 token 匹配时才清除 UserInfo 缓存，避免不同 token 的失败
	// 污染当前活跃 token 的 cachedUserInfo
	if sm.LoadToken() == token {
		sm.cachedUserInfo = nil
	}
}

// RecordSuccess 持锁记录激活成功，更新 token + backoff 清空 + 缓存 UserInfo。
// 调用方须持 sm.mu。
func (sm *sessionManager) RecordSuccess(token string, info *types.UserInfo) {
	sm.token.Store(token)
	sm.clearBackoff()
	if info != nil {
		sm.cachedUserInfo = info
	}
}

// SetBackoff 设置 backoff 窗口。
//
// 行为约定：
//   - d > 0：设置 backoff
//   - d <= 0：no-op（保持当前值），防止静默清零
//
// 设计一致：与 WithSessionBackoff 的「d<=0 拒绝」守卫对称。
// 公开方法 WithSessionBackoff 在 Option 层提供更详细的 warn 日志。
func (sm *sessionManager) SetBackoff(d time.Duration) {
	if d <= 0 {
		return
	}
	sm.mu.Lock()
	sm.backoff = d
	sm.mu.Unlock()
}

// tryActivate 在 sm.mu 持锁状态下执行 backoff 检查 + 4 步激活 + 状态记录。
//
// 调用方必须持 sm.mu 锁。
// 语义等价于原 Client.activateWithBackoffCheck，下沉到 sessionManager。
//
// 职责链：
//  1. 检查 ctx 是否已取消（优先于 backoff，避免 ctx 取消被掩盖为
//     ErrSessionBackoff）
//  2. backoff 检查（同 token 在窗口内 → 返回 ErrSessionBackoff）
//  3. 调用 activateFn 执行 4 步激活（持锁写 cookie jar）
//  4. 失败 → RecordFailure；成功 → RecordSuccess
func (sm *sessionManager) tryActivate(
	ctx context.Context,
	token string,
	activateFn func(context.Context, string) (*types.UserInfo, error),
) (*types.UserInfo, error) {
	// 先检查 ctx 是否已取消，优先于 backoff 检查。避免 ctx 已取消时
	// 被 backoff 窗口掩盖为 ErrSessionBackoff（backoff 在窗口内 → 返回
	// ErrSessionBackoff，调用方看到 ErrSessionBackoff 而非 context.Canceled，
	// 错误处理被误导）。
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// backoff 检查：上次失败且同 token 在窗口内 → 抑制
	if sm.isBackoffHit(token) {
		// errors.Join 同时包装 ErrSessionBackoff 和 sm.lastErr，保持
		// 错误链完整（errors.Is 可穿透到原始错误）。
		return nil, errors.Join(
			fmt.Errorf("%w: 上次 token %q 激活失败重试 %v 前，请稍后重试或换 token",
				ErrSessionBackoff, token, time.Since(sm.lastAttempt)),
			sm.lastErr,
		)
	}

	// 持锁激活：4 步串行执行，写 cookie jar 互斥
	info, err := activateFn(ctx, token)
	if err != nil {
		sm.RecordFailure(token, err)
		return nil, err
	}
	sm.RecordSuccess(token, info)
	return info, nil
}

// Activate 封装了 session 激活的 4 步 HTTP、backoff 检查和状态管理。
// 调用方负责传实际的 activateFn，便于隔离测试。
//
// 持锁 4 步契约：cookie jar 是 Client 级别共享资源，不同 token 的并发 4 步 HTTP
// 会竞态写入同一 cookie jar，破坏隔离性。保持锁内 HTTP 是最简单的正确方案。
//
// 对同 token：DCL fast path 保证只有首次 goroutine 持锁执行 4 步，
// 后续 goroutine 直接从缓存返回（不阻塞）。
// 对不同 token：串行激活（不会死锁，约 200-500ms 内释放）。
func (sm *sessionManager) Activate(
	ctx context.Context,
	token string,
	activateFn func(context.Context, string) (*types.UserInfo, error),
) (*types.UserInfo, error) {

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 锁内检查：token 已匹配且缓存非空 → 直接返回缓存
	// caveat: cachedUserInfo 可能为 nil（如上次 RecordFailure 清空但 token 未变），
	// 此时走 tryActivate 让 backoff 或 retry 处理，避免返回 (nil, nil) 混淆调用方。
	if sm.LoadToken() == token && sm.cachedUserInfo != nil {
		return sm.cachedUserInfo, nil
	}

	// 委托给 tryActivate：backoff 检查 + 激活 + 状态记录
	return sm.tryActivate(ctx, token, activateFn)
}
