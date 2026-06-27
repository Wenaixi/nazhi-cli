// 验证 task.go 中所有 ErrBusinessRejected 包装路径正确使用 errors.Join 模式，
// 保证 errors.Is(err, ErrBusinessRejected) 能够正确命中。
//
// 覆盖 3 个场景：
//   - fetchDimensions（通过 GetDimensions）→ 业务错误必须可识别
//   - SubmitTask → 业务错误必须可识别
//   - GetCircleTypeByTaskID → 业务错误必须可识别
//
// 统一断言：返回 err 必须通过 errors.Is(err, ErrBusinessRejected) 且不被误判为 ErrLoginRejected。
package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// assertErrBizRejected 统一断言：返回 err 必须 (1) 非 nil、(2) errors.Is 命中
// ErrBusinessRejected、(3) 不被误判为 ErrLoginRejected。
func assertErrBizRejected(t *testing.T, err error, caller string) {
	t.Helper()
	if err == nil {
		t.Fatalf("[%s] 期望业务错误，但得到 nil", caller)
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("[%s] 业务错误应包装为 ErrBusinessRejected（errors.Is 可命中），err=%v", caller, err)
	}
	if errors.Is(err, client.ErrLoginRejected) {
		t.Errorf("[%s] 业务错误不应被误判为 ErrLoginRejected，err=%v", caller, err)
	}
}

// TestErrWrapping_FetchDimensions 验证 fetchDimensions（通过 GetDimensions）
// 在 server 返回 code != 1 时，错误链包含 ErrBusinessRejected。
func TestErrWrapping_FetchDimensions(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getDimensions" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(500, "维度服务暂不可用", nil, nil)))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetDimensions(context.Background(), "test-token")
	assertErrBizRejected(t, err, "GetDimensions")
}

// TestErrWrapping_SubmitTask 验证 SubmitTask 在 server 返回 code != 1 时，
// 错误链包含 ErrBusinessRejected。
func TestErrWrapping_SubmitTask(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/addCircle" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(2, "任务已提交", nil, nil)))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.SubmitTask(context.Background(), "test-token", types.TaskSubmitPayload{
		CircleTaskID: 1,
		CircleTypeID: 1,
	})
	assertErrBizRejected(t, err, "SubmitTask")
}

// TestErrWrapping_GetCircleTypeByTaskID 验证 GetCircleTypeByTaskID 在 server
// 返回 code != 1 时，错误链包含 ErrBusinessRejected。
func TestErrWrapping_GetCircleTypeByTaskID(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getCircleTypeByTaskId" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(404, "任务类型不存在", nil, nil)))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetCircleTypeByTaskID(context.Background(), "test-token", 12345)
	assertErrBizRejected(t, err, "GetCircleTypeByTaskID")
}
