package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestGetMyInfo_BizError_WrapsErrBusinessRejected 验证 GetMyInfo 业务错误正确包装
// ErrBusinessRejected，让 errors.Is 能命中。
// RED 阶段（修复前）：getMyInfoRaw 用 %v 包装 errors.Join(ErrBusinessRejected, err)，
// 断开错误链 → errors.Is(err, ErrBusinessRejected) 返回 false。
// GREEN 阶段（修复后）：改用 %w → errors.Is 正常命中。
// 与 TestFindingE_GetMyInfo_BizError 的区别：该测试走 warmupBizHandler（步骤 4 成功
// 缓存 UserInfo → GetMyInfo 走 fast path 返回缓存，不调用 getMyInfoRaw），本测试
// 让步骤 4 返回 code=0，确保 getMyInfoRaw 路径被触发。
func TestGetMyInfo_BizError_WrapsErrBusinessRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"success"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":0,"msg":"用户信息缺失"}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	_, err := c.GetMyInfo(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望业务错误，但得到 nil")
	}

	// 关键断言：errors.Is 必须命中 ErrBusinessRejected
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("errors.Is(err, ErrBusinessRejected) 应为 true（%v 断开错误链），实际 err=%v", "%", err)
	}
}
