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
func TestSubmitTask_BusinessError_NotMisclassifiedAsLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
		WithTimeout(5*1000*1000*1000),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	// 跳过 session 激活，用 sm 直接注入已激活状态
	c.sm.StoreToken("fake-token")

	_, err = c.SubmitTask(context.Background(), "fake-token", types.TaskSubmitPayload{
		CircleTaskID: 1,
		CircleTypeID: 1,
	})

	if err == nil {
		t.Fatal("SubmitTask 业务错误应返回非 nil error")
	}

	if errors.Is(err, ErrLoginRejected) {
		t.Errorf("SubmitTask 业务错误不应被包装为 ErrLoginRejected，err=%v", err)
	}

	if !errors.Is(err, ErrBusinessRejected) {
		t.Errorf("SubmitTask 业务错误应被包装为 ErrBusinessRejected，err=%v", err)
	}
}

func ptr(s string) *string {
	return &s
}
