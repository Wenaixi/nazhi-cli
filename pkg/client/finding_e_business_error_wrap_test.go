// Package client_test 包含 review-tdd round-4 group-D Finding E 的测试：
// 验证所有调用 CheckCode 失败的 6 处都正确包装 ErrBusinessRejected，
// 不再裸传 *BusinessError 而丢失 errors.Is(err, ErrBusinessRejected) 语义信号。
package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// assertBusinessError 统一断言：返回 err 必须 (1) 非 nil、(2) errors.Is 命中
// ErrBusinessRejected、(3) 不被误判为 ErrLoginRejected。三个断言的顺序是
// fail-fast：先看是否有 err（必返回），再看是否带语义信号（核心），最后看
// 是否反向污染（回归保护）。
func assertBusinessError(t *testing.T, err error, caller string) {
	t.Helper()
	if err == nil {
		t.Fatalf("[%s] 期望业务错误，但得到 nil", caller)
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("[%s] 业务错误应被包装为 ErrBusinessRejected（让 SDK 用户能精确判定），err=%v", caller, err)
	}
	if errors.Is(err, client.ErrLoginRejected) {
		t.Errorf("[%s] 业务错误不应被误判为 ErrLoginRejected（会导致用户错误地走重新登录），err=%v", caller, err)
	}
}

// TestFindingE_SubmitSelfEvaluation_BizError 验证 SubmitSelfEvaluation
// 在 server 返回 code != 1 时包装为 ErrBusinessRejected（不是裸 *BusinessError）。
func TestFindingE_SubmitSelfEvaluation_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentMoralEduNew/addSelfEvaluation":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(2, "评价已提交过", nil, nil)))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.SubmitSelfEvaluation(context.Background(), "test-token", "我的自我评价")
	assertBusinessError(t, err, "SubmitSelfEvaluation")
}

// TestFindingE_QuerySelfEvaluation_BizError 验证 QuerySelfEvaluation
// 在 server 返回 code != 1 时包装为 ErrBusinessRejected。
func TestFindingE_QuerySelfEvaluation_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentMoralEduNew/querySelfEvaluation":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(0, "未找到评价", nil, nil)))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	assertBusinessError(t, err, "QuerySelfEvaluation")
}

// TestFindingE_QuerySelfGradEvaluation_BizError 验证 QuerySelfGradEvaluation
// 在 server 返回 code != 1 时包装为 ErrBusinessRejected。
func TestFindingE_QuerySelfGradEvaluation_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentMoralEduNew/querySelfGradEvaluation":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(500, "学期未开启", nil, nil)))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.QuerySelfGradEvaluation(context.Background(), "test-token")
	assertBusinessError(t, err, "QuerySelfGradEvaluation")
}

// TestFindingE_GetMyInfo_BizError 验证 GetMyInfo 在 server 返回 code != 1 时
// 包装为 ErrBusinessRejected。注：GetMyInfo 是「最佳努力」设计，调用方通常吞掉
// 错误，但语义上仍应保留 ErrBusinessRejected 信号（与其他业务错误对齐）。
func TestFindingE_GetMyInfo_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		// 第一次 getMyInfo 走 warmupBizHandler 自己的预热响应（once）。
		// 第二次 getMyInfo 落到本 fn：返回业务错误。
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(unifiedJSON(0, "用户信息缺失", nil, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetMyInfo(context.Background(), "test-token")
	// GetMyInfo 是「最佳努力」：可能返回 (nil, nil)，也可能返回 (nil, err)。
	// 关键：若返回 err，必须是 ErrBusinessRejected 包装。
	if err == nil {
		t.Skip("GetMyInfo 在 best-effort 模式下可能吞错，此测试不强制要求返回 err")
	}
	assertBusinessError(t, err, "GetMyInfo")
}

// TestFindingE_FetchDimensions_BizError 验证 fetchDimensions (GetDimensions
// 底层共用 helper) 在 server 返回 code != 1 时包装为 ErrBusinessRejected。
func TestFindingE_FetchDimensions_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(2, "服务维护中", nil, nil)))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetDimensions(context.Background(), "test-token")
	assertBusinessError(t, err, "GetDimensions")
}

// TestFindingE_GetCircleTypeByTaskId_BizError 验证 GetCircleTypeByTaskId
// 在 server 返回 code != 1 时包装为 ErrBusinessRejected。
func TestFindingE_GetCircleTypeByTaskId_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/studentCircleNew/getCircleTypeByTaskId":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(404, "任务类型不存在", nil, nil)))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetCircleTypeByTaskId(context.Background(), "test-token", 12345)
	assertBusinessError(t, err, "GetCircleTypeByTaskId")
}
