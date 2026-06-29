package client

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func TestSessionManager_SetBackoff(t *testing.T) {
	t.Run("sets positive duration", func(t *testing.T) {
		sm := newTestSM()
		sm.SetBackoff(30 * time.Second)
		if sm.backoff != 30*time.Second {
			t.Errorf("backoff = %v, want 30s", sm.backoff)
		}
	})

	t.Run("zero duration is no-op", func(t *testing.T) {
		sm := newTestSM()
		sm.backoff = 10 * time.Second
		sm.SetBackoff(0)
		if sm.backoff != 10*time.Second {
			t.Error("SetBackoff(0) 不应修改已有 backoff")
		}
	})

	t.Run("negative duration is no-op", func(t *testing.T) {
		sm := newTestSM()
		sm.backoff = 10 * time.Second
		sm.SetBackoff(-5 * time.Second)
		if sm.backoff != 10*time.Second {
			t.Error("SetBackoff(负数) 不应修改已有 backoff")
		}
	})
}

func TestSessionManager_TryActivate_Success(t *testing.T) {
	sm := newTestSM()
	var callCount int32
	expectedInfo := &types.UserInfo{Name: "success"}

	activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
		atomic.AddInt32(&callCount, 1)
		return expectedInfo, nil
	}

	info, err := sm.tryActivate(context.Background(), "tok", activateFn)
	if err != nil {
		t.Fatalf("tryActivate 应成功，err=%v", err)
	}
	if info != expectedInfo {
		t.Error("tryActivate 应返回 activateFn 的结果")
	}
	if n := atomic.LoadInt32(&callCount); n != 1 {
		t.Errorf("activateFn 应调 1 次，实际 %d", n)
	}
	if sm.LoadToken() != "tok" {
		t.Errorf("成功后 sm token = %q", sm.LoadToken())
	}
	if sm.lastErr != nil {
		t.Error("成功后 lastErr 应为 nil")
	}
}

func TestSessionManager_TryActivate_Failure(t *testing.T) {
	sm := newTestSM()
	errExpected := errors.New("activation failed")

	activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
		return nil, errExpected
	}

	info, err := sm.tryActivate(context.Background(), "tok", activateFn)
	if err == nil {
		t.Fatal("tryActivate 应返回 error")
	}
	if info != nil {
		t.Error("失败时 info 应为 nil")
	}
	if sm.lastErr == nil {
		t.Error("失败后 lastErr 应有值")
	}
	if sm.lastFailedToken != "tok" {
		t.Errorf("lastFailedToken = %q, want %q", sm.lastFailedToken, "tok")
	}
}

func TestSessionManager_TryActivate_BackoffHit(t *testing.T) {
	sm := newTestSM()
	sm.lastErr = errors.New("previous fail")
	sm.lastFailedToken = "tok"
	sm.lastAttempt = time.Now()
	sm.backoff = time.Hour // 长时间窗口

	activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
		t.Error("backoff 命中时不应调 activateFn")
		return nil, nil
	}

	_, err := sm.tryActivate(context.Background(), "tok", activateFn)
	if err == nil {
		t.Fatal("backoff hit 应返回 error")
	}
	if !errors.Is(err, ErrSessionBackoff) {
		t.Errorf("应包装 ErrSessionBackoff，实际: %v", err)
	}
}

// TestTryActivate_CtxCanceledOverridesBackoff 守护：ctx 取消时即使 backoff 窗口激活
// 也应返回 context.Canceled 而非 ErrSessionBackoff。
// Gap-1: 旧顺序先查 backoff 后查 ctx.Err()，ctx 取消被掩盖为 ErrSessionBackoff。
func TestTryActivate_CtxCanceledOverridesBackoff(t *testing.T) {
	sm := newTestSM()
	sm.lastErr = errors.New("previous fail")
	sm.lastFailedToken = "tok"
	sm.lastAttempt = time.Now()
	sm.backoff = time.Hour // 长时间窗口

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	activateFn := func(ctx context.Context, token string) (*types.UserInfo, error) {
		t.Error("ctx 已取消时不应调 activateFn")
		return nil, nil
	}

	_, err := sm.tryActivate(ctx, "tok", activateFn)
	if err == nil {
		t.Fatal("ctx 已取消应返回 error")
	}
	if errors.Is(err, ErrSessionBackoff) {
		t.Error("ctx 已取消时不应返回 ErrSessionBackoff，应返回 context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("应返回 context.Canceled，实际: %v", err)
	}
}
