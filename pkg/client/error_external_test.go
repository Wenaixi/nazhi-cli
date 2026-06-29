// error_external_test.go 聚合 errors 相关外部黑盒测试（package client_test）：
//   - GetMyInfo 业务错误包装
//   - fetchDimensions/SubmitTask 两处业务错误包装
//   - C10/B14: querySelfEval helper + errors.Join CheckCode
//   - E finding: Submit/Query SelfEvaluation 业务错误包装
//   - F finding: FetchTasks 单维度业务错误 propagate
package client_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── biz_error_wrap_getmyinfo_test.go: GetMyInfo 业务错误 ───

// TestGetMyInfo_BizError_WrapsErrBusinessRejected 验证 GetMyInfo 业务错误正确包装
// ErrBusinessRejected，让 errors.Is 能命中。
// RED 阶段（修复前）：getMyInfoRaw 用 %v 包装 errors.Join(ErrBusinessRejected, err)，
// 断开错误链 → errors.Is(err, ErrBusinessRejected) 返回 false。
// GREEN 阶段（修复后）：改用 %w → errors.Is 正常命中。
// 与 TestFindingE_GetMyInfo 的区别：该测试走 warmupBizHandler（步骤 4 成功
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

// ─── err_business_wrapping_test.go: 3 处业务错误包装 ───

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

// ─── finding_c10_b14_test.go: querySelfEval helper + errors.Join ───

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
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("errors.Is 应命中 ErrBusinessRejected，实际 %v", err)
	}
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

// ─── finding_e_business_error_wrap_test.go: 6 处业务错误包装 ───

// assertBusinessError 统一断言：返回 err 必须 (1) 非 nil、(2) errors.Is 命中
// ErrBusinessRejected、(3) 不被误判为 ErrLoginRejected。
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
}

// TestFindingE_GetMyInfo_BizError 验证 GetMyInfo 在 server 返回 code != 1 时
// 包装为 ErrBusinessRejected（与 TestGetMyInfo_BizError_WrapsErrBusinessRejected
// 使用不同的 mock 结构，确保全路径覆盖）。
//
// 此测试不依赖 warmupBizHandler（步骤 4 的 sync.Once 缓存会阻止错误路径触发），
// 改用裸 httptest.Server + client.New 直接对齐 getMyInfoRaw 路径。
func TestFindingE_GetMyInfo_BizError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, nil)))
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, nil)))
		case "/api/studentInfo/getMyInfo":
			// 步骤 4 返回 code=0 → ensureActivated 返回 err
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(0, "用户信息缺失", nil, nil)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	_, err := c.GetMyInfo(context.Background(), "test-token")
	if err == nil {
		t.Fatal("GetMyInfo 应返回业务错误，但返回 nil")
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

// ─── finding_f_bizerror_propagate_test.go: FetchTasks 单维度业务错误 propagate ───

// TestFindingF_FetchTasks_BizErrorPropagates 验证 FetchTasks 在某维度返回
// 业务错误时，错误不被静默吞咽，而是 propagate 为带 ErrBusinessRejected
// 信号的 error。
func TestFindingF_FetchTasks_BizErrorPropagates(t *testing.T) {
	const dimCount = 3
	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B-业务错误"},
		{"id": 3, "name": "成功C"},
	}

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			if dimID == "2" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(unifiedJSON(500, "维度服务暂时不可用", nil, nil)))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 2000, "name": "任务" + dimID, "circleTypeId": 9999, "hours": 1.0, "circleTaskStatus": "未提交", "upPic": 1},
			})))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	tasks, err := c.FetchTasks(context.Background(), "test-token")

	if err == nil {
		t.Fatal("FetchTasks 单维度业务错误应 propagate，不应被静默吞咽")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("FetchTasks 业务错误应被包装为 ErrBusinessRejected，err=%v", err)
	}
	if errors.Is(err, client.ErrLoginRejected) {
		t.Errorf("FetchTasks 业务错误不应被误判为 ErrLoginRejected，err=%v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("期望 2 个成功任务（维度 1 + 3），得到 %d", len(tasks))
	}
}

// TestFindingF_FetchTasks_BizErrorMsgContainsDimInfo 验证错误信息包含
// 失败维度的诊断信息（dim id / name），方便 SDK 用户排障。
func TestFindingF_FetchTasks_BizErrorMsgContainsDimInfo(t *testing.T) {
	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B-业务错误"},
	}

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			if dimID == "2" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(unifiedJSON(0, "维度无数据", nil, nil)))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 2000, "name": "任务" + dimID, "circleTypeId": 9999, "hours": 1.0, "circleTaskStatus": "未提交", "upPic": 1},
			})))
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.FetchTasks(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望业务错误")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "2") && !strings.Contains(errStr, "失败B-业务错误") {
		t.Errorf("错误信息应包含失败维度标识，err=%v", errStr)
	}
	if !strings.Contains(errStr, "维度无数据") {
		t.Errorf("错误信息应包含业务错误 msg，err=%v", errStr)
	}
}

// TestFindingF_FetchTasks_HTTPErrorStillLogDebug 验证 F 修复**没有过度泛化**：
// 单维度 HTTP 网络错误（如 500 连接错）仍走 logDebug，不影响整体拉取。
func TestFindingF_FetchTasks_HTTPErrorStillLogDebug(t *testing.T) {
	const dimCount = 3
	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B-HTTP500"},
		{"id": 3, "name": "成功C"},
	}

	var loggerCalls atomic.Int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			if dimID == "2" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 2000, "name": "任务" + dimID, "circleTypeId": 9999, "hours": 1.0, "circleTaskStatus": "未提交", "upPic": 1},
			})))
		}
	})))
	defer biz.Close()

	c, _ := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithLogger(slog.New(slog.NewTextHandler(testLogWriter{onWrite: func() {
			loggerCalls.Add(1)
		}}, &slog.HandlerOptions{Level: slog.LevelDebug}))),
	)

	tasks, err := c.FetchTasks(context.Background(), "test-token")
	if err == nil {
		t.Fatal("F2 修复后 HTTP/解析错误应 propagate 为 ErrBusinessRejected")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("HTTP/解析错误应包装为 ErrBusinessRejected（部分成功），err=%v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("期望 2 个成功任务（维度 1 + 3），得到 %d", len(tasks))
	}
	if loggerCalls.Load() == 0 {
		t.Error("HTTP 错误仍应触发 logDebug")
	}
}
