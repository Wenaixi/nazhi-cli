package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// defaultSessionBackoff 是激活失败后禁止重试的默认时间窗口。默认 5 秒。
const defaultSessionBackoff = 5 * time.Second

// ActivateSession 初始化目标平台业务 Session。
// 按真实抓包验证：必须按以下 4 步顺序激活，否则后续接口返回空数据：
//  1. GET /（首页）
//  2. GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
//  3. GET /api/studentInfo/getMenu（Referer: /home）
//  4. GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
//
// 返回用户基本信息（含座号）。4 步任一失败立即 propagate error：
// 步骤 4（getMyInfo）是 4 步契约的一部分，失败直接上抛，不做兜底降级。
//
// 内部实现：委托给 sm.Activate，由 sessionManager 负责持锁 fast path、
// backoff 检查、持锁 4 步激活和状态记录。学校信息 SSO 回退补全
// （postProcessSchoolFallback）在 sm.mu 解锁后执行——P1-B：网络往返
// 禁止进入临界区，否则最坏把锁窗口放大到 c.http.Timeout 秒级。
//
// 外部并发契约：
//   - 本函数委托给 sm.Activate，后者在 sm.mu 持锁状态下执行 4 步网络请求
//     （正常数百毫秒量级），持有锁期间不会回调外层或锁住其他互斥资源；
//     锁内无任何计划外网络往返（学校回退已移至锁外）。
//   - 外部调用方应**避免**在本函数持锁路径内嵌套其他锁，
//     否则可能引发 ABBA 死锁（如 errgroup.Go 中先持锁 A 再调本函数，
//     本函数持 sm.mu 时反调锁 A）。
//   - 外部使用模式：直接 goroutine 并发调本函数是安全的——sm.mu
//     只会序列化 4 步激活，不会让其他 goroutine 饿死（窗口为 4 步请求耗时，
//     受 c.http.Timeout 封顶；锁内不含 SSO 学校回退）。
//   - 如果需要在锁内调本函数（如 sync.Mutex 临界区），需确保外层锁
//     的获取/释放顺序一致，不会形成循环等待。
//
// 并发安全：本方法委托给 sm.Activate（持锁 fast path），同 token 二次调用
// 直接命中缓存不重复执行 4 步。
//
// Backoff 缓存：失败时通过 sm.RecordFailure 更新 lastErr / lastAttempt /
// lastFailedToken。CLI 路径（直接调 ActivateSession）与业务方法路径
// （通过 ActivateSession 间接调）共享同一份 backoff 缓存，
// 同 token 在窗口内的重复调用会被抑制。
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error) {
	info, err := c.sm.Activate(ctx, token, c.activateSessionLocked)
	if err != nil || info == nil {
		return info, err
	}
	// P1-B：学校信息 SSO 回退在 sm.mu 锁外执行（幂等：字段已齐则零开销直通）。
	//
	// P0-A7（十六域审计）：info 与 sm.cachedUserInfo 是同一指针（RecordSuccess 原指针入缓存），
	// 锁外直接 postProcessSchoolFallback(ctx, info) 会原地改共享缓存指针，与 fast path
	// 并发读取方形成数据竞争（Go 内存模型下 string 头撕裂风险）。
	// 修法：浅拷贝出 infoCopy，让 postProcessSchoolFallback 改副本；UpdateCachedUserInfo(infoCopy)
	// 把缓存指针替换为 infoCopy（不同指针但 token 一致 → 走替换分支）；fast path 命中后
	// 继续返回 infoCopy（同一指针），保持 DCL 同一缓存指针契约。原 info（步骤 4 网络响应对象）
	// 不再被并发读到。
	// P1-B + P0-A7（十六域审计）：学校信息 SSO 回退在 sm.mu 锁外执行。
	//
	// 修复要点：info 与 sm.cachedUserInfo 是同一指针（RecordSuccess 原指针入缓存），
	// 锁外直接 postProcessSchoolFallback(ctx, info) 会原地改共享缓存指针，与 fast path
	// 并发读取方形成数据竞争（Go 内存模型下 string 头撕裂风险）。
	//
	// 修法：用 sm.fallbackDone 原子标志判断当前缓存是否已 fallback 补完成。
	//   - false（首次激活或缓存被 RecordFailure 清空后）：浅拷贝出 infoCopy，让 fallback 改副本，
	//     UpdateCachedUserInfo(&infoCopy) 把缓存指针替换为 infoCopy；fallbackDone 设回 true。
	//     fast path 重入返回的也是 infoCopy（同一指针），保持 DCL 同一缓存指针契约。
	//   - true（上次已 fallback）：直接返回 info，缓存指针不变，DCL 同一指针契约保持。
	if !c.sm.fallbackDone.Load() {
		infoCopy := *info
		c.postProcessSchoolFallback(ctx, &infoCopy)
		c.sm.UpdateCachedUserInfo(&infoCopy, token)
		c.sm.fallbackDone.Store(true)
		return &infoCopy, nil
	}
	return info, nil
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

	// ponytail: 4-step HAR activation 硬编码，可 data-driven（[]activationStep{label, path, referer}），
	// payoff marginal，当前无重复步骤组，保持显式简单。

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
	if err := c.doGetMenu(ctx, menuURL, headers, step2Referer, "步骤2"); err != nil {
		return nil, err
	}

	// 步骤3：GET /api/studentInfo/getMenu（Referer: /home）
	if err := c.doGetMenu(ctx, menuURL, headers, c.bizURL("/home"), "步骤3"); err != nil {
		return nil, err
	}

	// 步骤4：GET /api/studentInfo/getMyInfo（获取完整个人资料，含 seat/号数）
	// 关键：用内部 getMyInfoRaw 而非公开 GetMyInfo，避免 sm.mu（sync.Mutex）
	// 持锁时再次进入死锁（不可重入限制）。
	// 失败 propagate：步骤 4 是 4 步 HAR 契约的一部分。
	return c.getMyInfoRaw(ctx, token)
}

// doGetMenu 执行一次 getMenu 请求并 drain 响应体。
//
// helper 抽取动机：ActivateSession 步骤 2/3 几乎完全相同（同样的 URL、
// 同样的方法、差异仅在 Referer），inline 实现重复 ~14 行。统一在此处理
// 头复制、drain+close 资源回收，调用方只关心 referer 与错误标签。
//
// 注意：baseHeaders 不会被修改——内部 clone 后再写入 Referer，
// 保证调用方的原始 map 不受副作用影响。
//
// stepLabel 是用于错误信息的人类可读标签（如 "步骤2" / "步骤3"），调用方
// 需自行保证唯一性以便错误诊断。
func (c *Client) doGetMenu(ctx context.Context, menuURL string, baseHeaders map[string]string, referer, stepLabel string) error {
	// clone 避免修改调用方传入的 map
	hdr := make(map[string]string, len(baseHeaders)+1)
	for k, v := range baseHeaders {
		hdr[k] = v
	}
	hdr["Referer"] = referer

	resp, err := c.rawDoWithResp(ctx, http.MethodGet, menuURL, nil, hdr, "")
	if err != nil {
		return fmt.Errorf("ActivateSession %s（getMenu）失败: %w", stepLabel, err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// 按 StatusCode 切换 sentinel 包装，让 SDK 用户能通过
		// errors.Is 精确识别原因（限流 / 服务端异常 / HTTP 层错误）。
		sentinel := classifyHTTPStatus(resp.StatusCode, ErrInvalidResponse)
		return fmt.Errorf("%w: ActivateSession %s getMenu 返回状态码 %d",
			sentinel, stepLabel, resp.StatusCode)
	}
	return nil
}

// ─── sessionManager: 业务 API session 激活状态机 ───
//
// 职责范围：
//   - 4 步 HAR 激活流程（ActivateSession）
//   - backoff 缓存（失败后冷却抑制 thundering herd）
//   - 持锁 fast path（sessionToken + cachedUserInfo）
//   - 内部 getMyInfo 缓存（GetMyInfo 复用步骤 4 数据）
//
// 并发安全：
//   - mu 保护所有状态变更（4 步激活写 cookie jar）
//   - token 经 atomic.Value 存取：读路径无锁，写路径持锁
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
	cachedUserInfo  *types.UserInfo // 持锁 fast path 缓存。CLI 单进程命中一次，
	// SDK 多 goroutine 并发 FetchTasks 可复用步骤 4 数据。
	fallbackDone atomic.Bool // 标记当前 cachedUserInfo 是否已走 postProcessSchoolFallback，
	// 锁外读安全。ActivateSession 据此跳过重复 fallback（重复 fallback 替换缓存指针会破坏 DCL 同一指针契约）。
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
// 内部 helper，仅在 sm.mu 持锁路径内调用。
func (sm *sessionManager) clearBackoff() {
	sm.lastErr = nil
	sm.lastFailedToken = ""
}

// Reset 清除 sessionManager 的 backoff 状态，供 Client.Close() 调用。
// 与 clearBackoff 的区别：本方法自行持锁，调用方无需关心并发安全。
func (sm *sessionManager) Reset() {
	sm.mu.Lock()
	sm.clearBackoff()
	sm.mu.Unlock()
}

// InvalidateCachedUserInfo 清空持锁 fast path 的 UserInfo 缓存。
// 持锁，调用方无需关心并发安全。
//
// 典型场景：UpdateMyInfo 成功后调用，避免后续 GetMyInfo 返回更新前的快照。
// 同步重置 fallbackDone：新缓存尚未经过学校回退，出口门控不得因残留标志跳过它。
func (sm *sessionManager) InvalidateCachedUserInfo() {
	sm.mu.Lock()
	sm.cachedUserInfo = nil
	sm.fallbackDone.Store(false)
	sm.mu.Unlock()
}

// StoreToken 持锁写 token，并清除 backoff 状态。
// 当前无生产调用方，仅供测试构造状态使用；生产路径经 RecordSuccess 写入。
func (sm *sessionManager) StoreToken(token string) {
	sm.token.Store(token)
	sm.clearBackoff()
}

// UpdateCachedUserInfo 持锁刷新 UserInfo 缓存（仅当当前 token 匹配时生效）。
// forToken 是触发本写入的 info 所属 token：仅当 sm.LoadToken() == forToken 才允许
// 用补全版本替换缓存——跨 token 的迟到写入（多 goroutine 场景）被显式忽略，
// 避免旧 token 的后处理污染新 token 的缓存。
// 两个调用方分别传入自己刚完成激活/拉取的 token，实现在锁内比对。
func (sm *sessionManager) UpdateCachedUserInfo(info *types.UserInfo, forToken string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if info == nil {
		return
	}
	if sm.cachedUserInfo == info {
		return // 同一指针：RecordSuccess 已写入，无需重复赋值
	}
	// 不同指针且 token 一致 → 用补全版本替换
	if sm.LoadToken() == forToken {
		sm.cachedUserInfo = info
	}
}

// RecordFailure 记录激活失败，按 token 匹配决定是否清缓存。
// 本方法仅在 tryActivate 的 mu 持锁路径内调用，不额外取锁。
func (sm *sessionManager) RecordFailure(token string, err error) {
	sm.lastErr = err
	sm.lastAttempt = time.Now()
	sm.lastFailedToken = token
	// 只有 token 匹配时才清除 UserInfo 缓存，避免不同 token 的失败
	// 污染当前活跃 token 的 cachedUserInfo
	if sm.LoadToken() == token {
		sm.cachedUserInfo = nil
		sm.fallbackDone.Store(false)
	}
}

// RecordSuccess 记录激活成功，更新 token + backoff 清空 + 缓存 UserInfo。
// 本方法仅在 tryActivate 的 mu 持锁路径内调用，不额外取锁。
func (sm *sessionManager) RecordSuccess(token string, info *types.UserInfo) {
	sm.token.Store(token)
	sm.clearBackoff()
	if info != nil {
		sm.cachedUserInfo = info
	}
	// 新缓存的学校回退尚未执行，重置标志让 ActivateSession 出口门控重新走补全分支
	// （否则换 token / 缓存重建后 SchoolID/SchoolName 静默为空，直到同 token 失败一次才自愈）。
	sm.fallbackDone.Store(false)
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
				ErrSessionBackoff, logx.RedactValue("token", token), time.Since(sm.lastAttempt)),
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
// 对同 token：locked fast path 保证只有首次 goroutine 持锁执行 4 步，
// 激活完成后从缓存返回，不再重复执行 4 步。
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
