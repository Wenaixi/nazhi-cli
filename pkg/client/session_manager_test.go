package client

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// newTestSM 构造一个测试用的 sessionManager，backoff 窗口压缩到 50ms 方便测试。
func newTestSM() *sessionManager {
	var sm sessionManager
	sm.backoff = 50 * time.Millisecond
	return &sm
}

func TestSessionManager_IsBackoffHit(t *testing.T) {
	t.Run("fresh sm returns false", func(t *testing.T) {
		sm := newTestSM()
		if sm.isBackoffHit("token") {
			t.Error("全新 sessionManager 不应命中 backoff")
		}
	})

	t.Run("same token within window returns true", func(t *testing.T) {
		sm := newTestSM()
		sm.lastErr = errors.New("some error")
		sm.lastFailedToken = "token"
		sm.lastAttempt = time.Now()

		if !sm.isBackoffHit("token") {
			t.Error("同 token 且在窗口内应命中 backoff")
		}
	})

	t.Run("expired window returns false after sleep", func(t *testing.T) {
		sm := newTestSM()
		sm.lastErr = errors.New("some error")
		sm.lastFailedToken = "token"
		sm.lastAttempt = time.Now()

		time.Sleep(60 * time.Millisecond) // 等待超过 50ms 的 backoff 窗口

		if sm.isBackoffHit("token") {
			t.Error("窗口过期后不应命中 backoff")
		}
	})

	t.Run("different token returns false", func(t *testing.T) {
		sm := newTestSM()
		sm.lastErr = errors.New("some error")
		sm.lastFailedToken = "tokenA"
		sm.lastAttempt = time.Now()

		if sm.isBackoffHit("tokenB") {
			t.Error("不同 token 不应命中 backoff")
		}
	})

	t.Run("lastErr nil returns false", func(t *testing.T) {
		sm := newTestSM()
		sm.lastFailedToken = "token"
		sm.lastAttempt = time.Now()
		// lastErr 默认 nil

		if sm.isBackoffHit("token") {
			t.Error("lastErr 为 nil 时不应命中 backoff")
		}
	})
}

func TestSessionManager_Activate_DCL(t *testing.T) {
	t.Run("first call activates, second call returns cached", func(t *testing.T) {
		sm := newTestSM()
		var callCount int32
		expectedInfo := &types.UserInfo{Name: "test"}

		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			atomic.AddInt32(&callCount, 1)
			return expectedInfo, nil
		}

		// 第一次调用：触发激活
		info1, err := sm.Activate(context.Background(), "token", activateFn)
		if err != nil {
			t.Fatalf("第一次激活应成功，err=%v", err)
		}
		if info1 != expectedInfo {
			t.Error("第一次激活应返回 activateFn 的结果")
		}
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Errorf("activateFn 应恰好调用 1 次，实际调了 %d 次", n)
		}

		// 第二次调用：走 fast path 直接返回缓存
		info2, err := sm.Activate(context.Background(), "token", activateFn)
		if err != nil {
			t.Fatalf("第二次激活应成功（缓存），err=%v", err)
		}
		if info2 != expectedInfo {
			t.Error("第二次激活应返回缓存的 UserInfo")
		}
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Errorf("activateFn 仍应只调 1 次，实际调了 %d 次", n)
		}
	})

	t.Run("different token calls activateFn again", func(t *testing.T) {
		sm := newTestSM()
		var callCount int32

		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			atomic.AddInt32(&callCount, 1)
			return &types.UserInfo{Name: token}, nil
		}

		// tokenA 激活
		_, _ = sm.Activate(context.Background(), "tokenA", activateFn)
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Fatalf("第一次应调 1 次，调了 %d 次", n)
		}

		// tokenB 激活 —— 不同 token，应再次调用 activateFn
		info, err := sm.Activate(context.Background(), "tokenB", activateFn)
		if err != nil {
			t.Fatalf("tokenB 激活应成功，err=%v", err)
		}
		if info == nil || info.Name != "tokenB" {
			t.Errorf("tokenB 应返回其自身的 info，got %+v", info)
		}
		if n := atomic.LoadInt32(&callCount); n != 2 {
			t.Errorf("不同 token 应再次调用 activateFn，实际调了 %d 次", n)
		}
	})
}

func TestSessionManager_RecordFailure(t *testing.T) {
	t.Run("failure clears cachedUserInfo for same token", func(t *testing.T) {
		sm := newTestSM()
		sm.token.Store("token")
		sm.cachedUserInfo = &types.UserInfo{Name: "test"}

		sm.RecordFailure("token", errors.New("activation failed"))

		if sm.cachedUserInfo != nil {
			t.Error("同 token 失败后 cachedUserInfo 应被清空")
		}
	})

	t.Run("failure does NOT clear cache for different token", func(t *testing.T) {
		sm := newTestSM()
		sm.token.Store("activeToken")
		expectedInfo := &types.UserInfo{Name: "test"}
		sm.cachedUserInfo = expectedInfo

		sm.RecordFailure("otherToken", errors.New("activation failed"))

		if sm.cachedUserInfo != expectedInfo {
			t.Error("不同 token 失败不应清空当前 token 的缓存")
		}
	})
}

func TestSessionManager_StoreToken(t *testing.T) {
	t.Run("StoreToken sets token and clears backoff state", func(t *testing.T) {
		sm := newTestSM()

		sm.StoreToken("abc")

		if got := sm.LoadToken(); got != "abc" {
			t.Errorf("LoadToken 应返回 'abc'，实际 %q", got)
		}
		if sm.lastErr != nil {
			t.Error("StoreToken 后 lastErr 应为 nil")
		}
		if sm.lastFailedToken != "" {
			t.Errorf("StoreToken 后 lastFailedToken 应为空，实际 %q", sm.lastFailedToken)
		}
	})
}

func TestSessionManager_Activate_Backoff(t *testing.T) {
	t.Run("failure triggers backoff for same token", func(t *testing.T) {
		sm := newTestSM()
		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			return nil, errors.New("network error")
		}

		// 第一次：失败
		_, err1 := sm.Activate(context.Background(), "token", activateFn)
		if err1 == nil {
			t.Fatal("第一次激活应失败")
		}

		// 第二次：命中 backoff
		_, err2 := sm.Activate(context.Background(), "token", activateFn)
		if err2 == nil {
			t.Fatal("第二次激活应命中 backoff")
		}
		if !errors.Is(err2, ErrSessionBackoff) {
			t.Errorf("backoff 错误应包装 ErrSessionBackoff 哨兵，实际: %v", err2)
		}
	})

	t.Run("different token bypasses backoff", func(t *testing.T) {
		sm := newTestSM()
		var callCount int32
		activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
			atomic.AddInt32(&callCount, 1)
			if token == "tokenA" {
				return nil, errors.New("network error")
			}
			return &types.UserInfo{Name: token}, nil
		}

		// tokenA 失败
		_, _ = sm.Activate(context.Background(), "tokenA", activateFn)
		if n := atomic.LoadInt32(&callCount); n != 1 {
			t.Fatalf("tokenA 应调 1 次 activateFn，实际 %d", n)
		}

		// tokenB —— 不同 token，应绕过 backoff 再次调用 activateFn
		info, err := sm.Activate(context.Background(), "tokenB", activateFn)
		if err != nil {
			t.Fatalf("tokenB 应绕过 backoff 并成功，err=%v", err)
		}
		if info == nil || info.Name != "tokenB" {
			t.Errorf("tokenB 应返回其自身的 info，got %+v", info)
		}
		if n := atomic.LoadInt32(&callCount); n != 2 {
			t.Errorf("tokenB 应再次调用 activateFn，实际调了 %d 次", n)
		}
	})
}
