package client

import (
	"errors"
	"testing"
	"time"
)

// sessionManager 的 backoff 逻辑可以脱离 HTTP 做纯逻辑测试：
// isBackoffHit 只依赖 lastErr / lastFailedToken / lastAttempt / backoff
// 四个字段，无需真实的 HTTP 请求或 Client 引用。
//
// TDD 三步：
//   RED:   本测试引用 sessionManager 及其字段/方法 → 编译错误
//   GREEN: sessionManager 定义 struct + isBackoffHit 方法 → 测试通过
//   REFACTOR: 将 Client 的 session 字段迁移到 sessionManager

func TestSessionManagerBackoffHit(t *testing.T) {
	now := time.Now()
	sm := &sessionManager{
		backoff:         5 * time.Second,
		lastErr:         errors.New("模拟失败"),
		lastAttempt:     now,
		lastFailedToken: "token-A",
	}

	// Case 1: 同 token 且在窗口内 → 命中
	if !sm.isBackoffHit("token-A") {
		t.Error("isBackoffHit(token-A) 应返回 true（同 token 且在 5s 窗口内）")
	}

	// Case 2: 不同 token → 不命中
	if sm.isBackoffHit("token-B") {
		t.Error("isBackoffHit(token-B) 应返回 false（不同 token）")
	}

	// Case 3: 窗口外 → 不命中
	sm.lastAttempt = now.Add(-10 * time.Second)
	if sm.isBackoffHit("token-A") {
		t.Error("isBackoffHit(token-A) 应返回 false（超过 5s 窗口）")
	}

	// Case 4: lastErr == nil → 不命中
	sm.lastErr = nil
	sm.lastAttempt = now
	if sm.isBackoffHit("token-A") {
		t.Error("isBackoffHit(token-A) 应返回 false（lastErr == nil）")
	}
}

func TestSessionManagerBackoffDefault(t *testing.T) {
	// backoff <= 0 时使用 defaultSessionBackoff(5s)
	now := time.Now()
	sm := &sessionManager{
		backoff:         0, // 使用默认值
		lastErr:         errors.New("err"),
		lastAttempt:     now,
		lastFailedToken: "token-A",
	}

	if !sm.isBackoffHit("token-A") {
		t.Error("backoff=0 时应使用默认 5s 窗口，同 token 应命中")
	}

	// 窗口外
	sm.lastAttempt = now.Add(-10 * time.Second)
	if sm.isBackoffHit("token-A") {
		t.Error("backoff=0 时使用默认 5s，超过窗口应不命中")
	}
}
