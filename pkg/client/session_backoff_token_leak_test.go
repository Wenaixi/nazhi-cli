package client

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestSessionBackoffErrorMessage_DoesNotContainFullToken 回归测试：
// backoff 命中的错误消息不得内嵌完整 JWT token。
//
// 背景（十三域审计 P1）：session.go 曾以 %q 把完整 token 拼进错误消息，
// 经 CLI printError 漏斗直达 stderr 信封，task list 的 207 Partial 分支
// 更送进 stdout 数据流。token 对接收者无诊断价值（用户自己传入），
// 且 redact.go 的 tokenQueryRe/kvRe 均不匹配 %q 引号包裹形态，
// 错误信封通道完全绕开脱敏设施。
//
// 修复后：token 经 logx.RedactValue 掩蔽（保留首尾诊断价值），
// errors.Is 对 ErrSessionBackoff 与原始错误的穿透能力不受影响
// （后者由 TestSessionBackoff_ErrorsIsPenetratesToOriginalErr 单独锁定）。
func TestSessionBackoffErrorMessage_DoesNotContainFullToken(t *testing.T) {
	sm := newTestSM()

	// 用足够长且可识别的假 token 模拟真实 JWT 形态
	const fakeToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"
	sm.lastErr = ErrNetwork
	sm.lastFailedToken = fakeToken
	sm.lastAttempt = time.Now()
	sm.backoff = time.Hour // 长窗口确保命中

	_, err := sm.tryActivate(context.Background(), fakeToken, func(ctx context.Context, token string) (*types.UserInfo, error) {
		t.Error("backoff 命中时不应调用 activateFn")
		return nil, nil
	})
	if err == nil {
		t.Fatal("backoff 命中应返回 error")
	}

	msg := err.Error()
	if strings.Contains(msg, fakeToken) {
		t.Errorf("错误消息不得包含完整 token，实际: %q", msg)
	}
	// 掩蔽形态应保留可辨识痕迹而非整段消失（RedactValue 掩码含 ***）
	if !strings.Contains(msg, "***") {
		t.Errorf("错误消息应包含掩蔽标记 ***，实际: %q", msg)
	}
	if !errors.Is(err, ErrSessionBackoff) {
		t.Errorf("必须保留 ErrSessionBackoff 包装，err=%v", err)
	}
}
