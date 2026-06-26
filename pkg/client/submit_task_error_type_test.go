package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestSubmitTask_BusinessError_NotMisclassifiedAsLogin 验证 SubmitTask
// 在 server 返回业务错误（code != 1）时，包装的错误不应被 errors.Is 识别为
// ErrLoginRejected。
//
// F7 修复动机：原实现用 ErrLoginRejected 包装业务错误，导致 SDK 用户
// 按 README 推荐 errors.Is(err, ErrLoginRejected) 判定后错误地走重新登录
// 流程（业务错误其实与登录无关）。新实现用专门的 ErrBusinessRejected 哨兵。
func TestSubmitTask_BusinessError_NotMisclassifiedAsLogin(t *testing.T) {
	// mock server：返回 code=500 的业务错误（模拟"任务已提交"或"参数错"等）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 业务错误：code != 1
		resp := types.UnifiedResponse{
			Code: 500,
			Msg:  ptr("任务已提交或参数错误"),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c, err := New(
		WithBaseURL(server.URL),
		WithSSOBase(server.URL),
		WithTimeout(5*1000*1000*1000), // 5s
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	// 跳过 session 激活（用空 token 直接发请求，ActivateSession 会因
	// httptest server 不识别而失败——但我们要测的是 SubmitTask 业务错误包装）
	// 直接调用 SubmitTask 会触发 activateSessionIfNeeded；先跳过：
	// 用一个提前注入好的"已激活"标记，绕过 session 预热。
	c.sessionMu.Lock()
	c.sessionToken.Store("fake-token")
	c.sessionMu.Unlock()

	_, err = c.SubmitTask(context.Background(), "fake-token", types.TaskSubmitPayload{
		CircleTaskID: 1,
		CircleTypeID: 1,
	})

	// 必须返回错误
	if err == nil {
		t.Fatal("SubmitTask 业务错误应返回非 nil error")
	}

	// F7 关键断言：错误不应被识别为 ErrLoginRejected
	if errors.Is(err, ErrLoginRejected) {
		t.Errorf("SubmitTask 业务错误不应被包装为 ErrLoginRejected（用户会误以为要重新登录），err=%v", err)
	}

	// 必须被识别为 ErrBusinessRejected
	if !errors.Is(err, ErrBusinessRejected) {
		t.Errorf("SubmitTask 业务错误应被包装为 ErrBusinessRejected（让用户能精确判定），err=%v", err)
	}
}

func ptr(s string) *string {
	return &s
}
