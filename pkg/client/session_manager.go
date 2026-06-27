package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// sessionManager 管理业务 API session 的激活状态机。
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
type sessionManager struct {
	mu      sync.Mutex
	token   atomic.Value // 存储 string，写路径在 mu 持锁状态下完成
	backoff time.Duration

	lastErr         error
	lastAttempt     time.Time
	lastFailedToken string
	cachedUserInfo  *types.UserInfo
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
func (sm *sessionManager) LoadToken() string {
	v := sm.token.Load()
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// StoreToken 持锁写 token，并清除 backoff 状态。
// 调用方须持 sm.mu。
func (sm *sessionManager) StoreToken(token string) {
	sm.token.Store(token)
	sm.lastErr = nil
	sm.lastFailedToken = ""
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
	sm.lastErr = nil
	sm.lastFailedToken = ""
	if info != nil {
		sm.cachedUserInfo = info
	}
}

// GetCachedUserInfo 返回缓存的 UserInfo（fast path 用）。
func (sm *sessionManager) GetCachedUserInfo() *types.UserInfo {
	return sm.cachedUserInfo
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
	sm.backoff = d
}

// tryActivate 在 sm.mu 持锁状态下执行 backoff 检查 + 4 步激活 + 状态记录。
//
// 调用方必须持 sm.mu 锁。
// 语义等价于原 Client.activateWithBackoffCheck，下沉到 sessionManager。
//
// 职责链：
//  1. backoff 检查（同 token 在窗口内 → 返回 ErrSessionBackoff）
//  2. 调用 activateFn 执行 4 步激活（持锁写 cookie jar）
//  3. 失败 → RecordFailure；成功 → RecordSuccess
func (sm *sessionManager) tryActivate(
	ctx context.Context,
	token string,
	activateFn func(context.Context, string) (*types.UserInfo, error),
) (*types.UserInfo, error) {
	// backoff 检查：上次失败且同 token 在窗口内 → 抑制
	if sm.isBackoffHit(token) {
		return nil, fmt.Errorf("%w: 上次 token %q 激活失败重试 %v 前，请稍后重试或换 token: %v",
			ErrSessionBackoff, token, time.Since(sm.lastAttempt), sm.lastErr)
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

// Activate wraps the 4-step activation, DCL fast path, backoff check, and state management.
// 调用方负责传实际的 activateFn，便于隔离测试。
func (sm *sessionManager) Activate(
	ctx context.Context,
	token string,
	activateFn func(context.Context, string) (*types.UserInfo, error),
) (*types.UserInfo, error) {

	// fast path：token 已匹配 → 直接返回缓存，零开销
	if sm.LoadToken() == token {
		return sm.cachedUserInfo, nil
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 锁内重检
	if sm.LoadToken() == token {
		return sm.cachedUserInfo, nil
	}

	// 委托给 tryActivate：backoff 检查 + 激活 + 状态记录
	return sm.tryActivate(ctx, token, activateFn)
}
