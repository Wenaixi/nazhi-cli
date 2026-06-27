package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestGetMyInfoRaw_EmptyResponse_ReturnsErrEmptyUserInfo 回归测试 F10：
// getMyInfoRaw 在 returnData + dataMap 都为 nil 时（业务成功响应但确实无用户数据），
// 必须返回 (nil, ErrEmptyUserInfo) 而非 (nil, nil)。
// 修复前：返回 (nil, nil) → cmd/nazhi/session.go:38 printJSON(info) 输出裸 null
// 与 cmd/nazhi/whoami.go 的 {status: empty, reason: get_my_info_empty} envelope 不一致。
// 修复后：返回 (nil, ErrEmptyUserInfo) → cmd 层用 errors.Is 分支统一走 status envelope。
func TestGetMyInfoRaw_EmptyResponse_ReturnsErrEmptyUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 关键：code=1（业务成功）但 returnData + dataMap 都为 nil
		// 这是「服务端确认查询成功但确实无数据」的状态，不是错误
		_, _ = w.Write([]byte(`{"code":1,"returnData":null,"dataMap":null}`))
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.getMyInfoRaw(context.Background(), "test-token")

	// 关键断言：返回 nil UserInfo + ErrEmptyUserInfo 哨兵
	if info != nil {
		t.Errorf("空响应应返回 nil UserInfo，实际: %v", info)
	}
	if err == nil {
		t.Fatal("空响应应返回 ErrEmptyUserInfo，实际 nil")
	}
	if !errors.Is(err, ErrEmptyUserInfo) {
		t.Errorf("空响应错误必须包装 ErrEmptyUserInfo 哨兵，让 SDK 用户 errors.Is 识别。err=%v", err)
	}
}

// TestGetMyInfoRaw_ValidResponse_ReturnsUserInfo 验证正常响应仍返回 UserInfo + nil err。
// 防止 F10 修复破坏 happy path。
func TestGetMyInfoRaw_ValidResponse_ReturnsUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.getMyInfoRaw(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("正常响应不应返回 err，实际: %v", err)
	}
	if info == nil {
		t.Fatal("正常响应应返回 UserInfo，实际 nil")
	}
	if info.Name != "张三" {
		t.Errorf("Name = %q, 期望 %q", info.Name, "张三")
	}
}

// TestGetMyInfo_EmptyResponse_BestEffortReturnsNil 验证公开 GetMyInfo 仍然保持
// 「最佳努力设计」契约：调用方通常吞错，nil 不算 error。
// 但当 getMyInfoRaw 返回 (nil, ErrEmptyUserInfo) 时，GetMyInfo 应 propagate
// 这个 err（语义信号），让调用方能 errors.Is 分支处理空响应。
func TestGetMyInfo_EmptyResponse_PropagatesErrEmptyUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	info, err := c.GetMyInfo(context.Background(), "test-token")
	if info != nil {
		t.Errorf("空响应 GetMyInfo 应返回 nil UserInfo，实际: %v", info)
	}
	if !errors.Is(err, ErrEmptyUserInfo) {
		t.Errorf("空响应 GetMyInfo 必须 propagate ErrEmptyUserInfo，让 cmd 层按 errors.Is 分支。err=%v", err)
	}
	// 防止 dev 误把 ErrEmptyUserInfo 当成 ErrBusinessRejected 包装
	if errors.Is(err, ErrBusinessRejected) {
		t.Error("ErrEmptyUserInfo 不应被包装为 ErrBusinessRejected — 空响应不是业务错误")
	}
}

// 避免 types 包未使用（编译时静态检查）
var _ = types.UserInfo{}
