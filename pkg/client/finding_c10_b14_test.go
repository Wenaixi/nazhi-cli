// Package client_test 包含 review-tdd C4/C10/B14 的测试：
// - C4: response.go decodeField / decodeFieldSlice helper 提取（外部测试仍调用公开 API）
// - C10: self_eval.go querySelfEval 泛型 helper（测试 QuerySelfEvaluation / QuerySelfGradEvaluation 正确性）
// - B14: errors.Join 让 ErrBusinessRejected 同时支持 errors.Is 和 errors.As
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

// ─── C10: self_eval.go querySelfEval helper ───

// TestC10_QuerySelfEvaluation_Success 验证 QuerySelfEvaluation 正常路径。
func TestC10_QuerySelfEvaluation_Success(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfEvaluation" {
			t.Errorf("期望路径 querySelfEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
			"student_comment": "表现很好",
			"teacher_comment": "继续努力",
		}, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	status, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("QuerySelfEvaluation 失败: %v", err)
	}
	if status.StudentComment != "表现很好" {
		t.Errorf("期望 StudentComment=表现很好, 得到 %s", status.StudentComment)
	}
	if status.TeacherComment != "继续努力" {
		t.Errorf("期望 TeacherComment=继续努力, 得到 %s", status.TeacherComment)
	}
}

// TestC10_QuerySelfGradEvaluation_Success 验证 QuerySelfGradEvaluation 正常路径。
func TestC10_QuerySelfGradEvaluation_Success(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentMoralEduNew/querySelfGradEvaluation" {
			t.Errorf("期望路径 querySelfGradEvaluation, 得到 %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(1, "成功", map[string]any{
			"graduated": true,
		}, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	grad, err := c.QuerySelfGradEvaluation(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("QuerySelfGradEvaluation 失败: %v", err)
	}
	if grad == nil {
		t.Fatal("期望非 nil 结果")
	}
	if (*grad)["graduated"] != true {
		t.Errorf("期望 graduated=true, 得到 %v", (*grad)["graduated"])
	}
}

// ─── B14: errors.Join CheckCode 包装 ───

// TestB14_SubmitSelfEvaluation_ErrorsJoin 验证 SubmitSelfEvaluation 的 CheckCode
// 错误用 errors.Join 包装后，同时支持 errors.Is(ErrBusinessRejected) 和
// errors.As(*types.BusinessError)。
func TestB14_SubmitSelfEvaluation_ErrorsJoin(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(2, "已提交过", nil, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.SubmitSelfEvaluation(context.Background(), "test-token", "评价")
	if err == nil {
		t.Fatal("期望 err，得到 nil")
	}
	// errors.Is 必须命中
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("errors.Is 应命中 ErrBusinessRejected，实际 %v", err)
	}
	// errors.As 也必须可达（BusinessError 细节）
	var bizErr *types.BusinessError
	if !errors.As(err, &bizErr) {
		t.Errorf("errors.As 应拿到 *BusinessError，实际 %T", err)
	} else if bizErr.Code != 2 {
		t.Errorf("期望 Code=2，得到 %d", bizErr.Code)
	}
}

// TestB14_QuerySelfEvaluation_ErrorsJoin 验证 QuerySelfEvaluation 的 CheckCode
// 错误同样用 errors.Join 包装。
func TestB14_QuerySelfEvaluation_ErrorsJoin(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(0, "未找到", nil, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.QuerySelfEvaluation(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望 err，得到 nil")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("errors.Is 应命中 ErrBusinessRejected，实际 %v", err)
	}
	var bizErr *types.BusinessError
	if !errors.As(err, &bizErr) {
		t.Errorf("errors.As 应拿到 *BusinessError，实际 %T", err)
	} else if bizErr.Code != 0 {
		t.Errorf("期望 Code=0，得到 %d", bizErr.Code)
	}
}

// TestB14_QuerySelfGradEvaluation_ErrorsJoin 验证 QuerySelfGradEvaluation 的 CheckCode
// 错误同样用 errors.Join 包装。
func TestB14_QuerySelfGradEvaluation_ErrorsJoin(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(unifiedJSON(500, "学期未开启", nil, nil)))
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.QuerySelfGradEvaluation(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望 err，得到 nil")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("errors.Is 应命中 ErrBusinessRejected，实际 %v", err)
	}
	var bizErr *types.BusinessError
	if !errors.As(err, &bizErr) {
		t.Errorf("errors.As 应拿到 *BusinessError，实际 %T", err)
	} else if bizErr.Code != 500 {
		t.Errorf("期望 Code=500，得到 %d", bizErr.Code)
	}
}
