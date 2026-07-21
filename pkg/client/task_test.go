// task_test.go 聚合 task.go 内部白盒测试（package client）：
//   - G2: fetchTasksForDimensionSafe panic recover
//   - SubmitTask 业务错误分类
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── task_panic_recover_test.go (G2): panic recover ───

// TestFetchTasksForDimensionSafe_RecoversFromPanic 白盒验证 G2 修复。
// 构造路径：c.http=nil → fetchTasksForDimension 内部 c.activateSessionIfNeeded
// 调用 c.http.Do() → nil pointer dereference panic。
// fetchTasksForDimensionSafe 的 defer recover 把 panic 转成 error 返回。
// 旧行为：g.Go 闭包内 panic 逃逸 → 进程崩溃 → g.Wait 永不返回 → 测试进程崩溃。
// 新行为：panic 被捕获 → 返回 (nil, error) → 调用方拿到 error 进 dimErrs。
func TestFetchTasksForDimensionSafe_RecoversFromPanic(t *testing.T) {
	// 构造 *Client：ssoBaseURL/baseURL 有效但 http=nil（零值）
	// 触发 fetchTasksForDimension 内部 c.activateSessionIfNeeded → c.http.Do() → nil deref panic
	c := &Client{
		ssoBaseURL: "http://example.com",
		baseURL:    "http://example.com",
	}
	dim := types.Dimension{ID: 1, Name: "panic-dim"}
	headers := map[string]string{"X-Auth-Token": "test-token"}

	// 关键断言 1：调用不 panic 到 runtime（说明 recover 起作用）
	// 关键断言 2：返回 (nil, error)，error 信息含 "panic"
	tasks, err := c.fetchTasksForDimensionSafe(context.Background(), dim, headers)
	if tasks != nil {
		t.Errorf("期望 tasks=nil，实际 %v", tasks)
	}
	if err == nil {
		t.Fatal("期望 panic 转成 error 返回，实际 nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("err 应含 panic 信息，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "panic-dim") {
		t.Errorf("err 应含 dim 名称 panic-dim 便于定位，实际: %v", err)
	}
	t.Logf("G2 修复验证：panic 被 recover 捕获，err=%v", err)
}

// ─── submit_task_error_type_test.go: SubmitTask 业务错误分类 ───

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

	_, err = c.SubmitTask(context.Background(), "fake-token", types.TaskSubmitInput{
		TaskID:  1,
		Content: "测试内容",
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

// ─── appendLocked helper 测试 ───

// TestAppendLocked_ConcurrentSingleItem 验证 appendLocked 在 N 个 goroutine 并发
// 追加单元素场景下无 race 且结果正确（执行 `go test -race` 验证）。
func TestAppendLocked_ConcurrentSingleItem(t *testing.T) {
	var mu sync.Mutex
	var items []int
	var wg sync.WaitGroup

	const goroutines = 100
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			appendLocked(&mu, &items, n)
		}(i)
	}
	wg.Wait()

	if len(items) != goroutines {
		t.Errorf("期望 %d 个元素, 得到 %d", goroutines, len(items))
	}
}

// TestAppendLocked_ConcurrentVariadic 验证 appendLocked 变长追加在 N 个 goroutine 并发
// 追加切片场景下无 race 且结果正确。
func TestAppendLocked_ConcurrentVariadic(t *testing.T) {
	var mu sync.Mutex
	var items []int
	var wg sync.WaitGroup

	const goroutines = 50
	const batchSize = 3
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			appendLocked(&mu, &items, 1, 2, 3)
		}()
	}
	wg.Wait()

	expected := goroutines * batchSize
	if len(items) != expected {
		t.Errorf("期望 %d 个元素, 得到 %d", expected, len(items))
	}
}

// ─── TermName 独立字段 ───

// TestBuildTaskPayload_TermNameIndependent 验证 termName 不再误用 circleDate。
// 仅传 CircleDate 时 TermName 必须为空；显式传 TermName 时两者互不覆盖。
func TestBuildTaskPayload_TermNameIndependent(t *testing.T) {
	// mock：getCircleTypeByTaskId + getMyInfo（经 session 预热）
	var captured types.TaskAddCirclePayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"班会","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"","type":10}}`))
		case "/api/studentCircleNew/addCircle":
			_ = json.NewDecoder(r.Body).Decode(&captured)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL), WithTimeout(5*1000*1000*1000))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	// 只传 CircleDate → TermName 必须为空（当前 bug 会把 CircleDate 填进 TermName）
	_, err = c.SubmitTask(context.Background(), "tok", types.TaskSubmitInput{
		TaskID:     1001,
		Content:    "内容",
		CircleDate: "2026-03-01",
	})
	if err != nil {
		t.Fatalf("SubmitTask 失败: %v", err)
	}
	if captured.CircleDate != "2026-03-01" {
		t.Errorf("CircleDate 应保持 2026-03-01，实际 %q", captured.CircleDate)
	}
	if captured.TermName != "" {
		t.Errorf("仅传 CircleDate 时 TermName 应为空，实际 %q（不应回落为 CircleDate）", captured.TermName)
	}
}

// ─── isContextError helper 测试 ───

// TestIsContextError 验证 isContextError 对 context.Canceled、context.DeadlineExceeded 返回 true，
// 对其他错误返回 false。
func TestIsContextError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"context.Canceled", context.Canceled, true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"wrapped context.Canceled", fmt.Errorf("wrap: %w", context.Canceled), true},
		{"nil error", nil, false},
		{"other error", errors.New("some other error"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContextError(tt.err)
			if got != tt.want {
				t.Errorf("isContextError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ─── parseHours：对齐前端 hoursStatus（任务 hours>0 只读自动填；否则用户可填且常必填）───

// TestParseHours_InvalidNonEmpty 非空且 ParseFloat 失败应返回 ErrInvalidPayload。
func TestParseHours_InvalidNonEmpty(t *testing.T) {
	_, err := parseHours("abc", 1.5)
	if err == nil {
		t.Fatal("非法 hours 应返回 error")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("应 errors.Is(ErrInvalidPayload)，实际: %v", err)
	}
}

// TestParseHours_EmptyFallsBackWhenMetaPositive 空串且任务预设 >0 → 用元数据（前端只读自动填）。
func TestParseHours_EmptyFallsBackWhenMetaPositive(t *testing.T) {
	got, err := parseHours("  ", 2.5)
	if err != nil {
		t.Fatalf("空串+meta>0 不应 error: %v", err)
	}
	if got != 2.5 {
		t.Fatalf("期望 2.5，得到 %v", got)
	}
}

// TestParseHours_EmptyMetaZeroRequiresUser 任务预设 ≤0 且用户空 → 对齐前端 checkData 拦截。
func TestParseHours_EmptyMetaZeroRequiresUser(t *testing.T) {
	_, err := parseHours("", 0)
	if err == nil {
		t.Fatal("meta.Hours=0 且用户空应返回 error")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("应 errors.Is(ErrInvalidPayload)，实际: %v", err)
	}
	_, err = parseHours("  ", 0)
	if err == nil || !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("空白串+meta=0 也应 ErrInvalidPayload，实际: %v", err)
	}
}

// TestParseHours_ExplicitOverridesMeta 用户显式 hours 优先于任务预设。
func TestParseHours_ExplicitOverridesMeta(t *testing.T) {
	got, err := parseHours("3.5", 1.0)
	if err != nil {
		t.Fatalf("合法 hours 不应 error: %v", err)
	}
	if got != 3.5 {
		t.Fatalf("期望 3.5，得到 %v", got)
	}
	// 任务无预设时用户手填也应成功
	got, err = parseHours("1.5", 0)
	if err != nil {
		t.Fatalf("meta=0 但用户填了不应 error: %v", err)
	}
	if got != 1.5 {
		t.Fatalf("期望 1.5，得到 %v", got)
	}
}

// TestSubmitTask_InvalidHours 端到端：非法 hours 在 SubmitTask 路径返回 ErrInvalidPayload。
func TestSubmitTask_InvalidHours(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"班会","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"","type":10}}`))
		case "/api/studentCircleNew/addCircle":
			t.Error("非法 hours 不应到达 addCircle")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL), WithTimeout(5*1000*1000*1000))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.SubmitTask(context.Background(), "tok", types.TaskSubmitInput{
		TaskID:  1001,
		Content: "内容",
		Hours:   "not-a-number",
	})
	if err == nil {
		t.Fatal("非法 hours 应失败")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("应 errors.Is(ErrInvalidPayload)，实际: %v", err)
	}
}

// TestSubmitTask_EmptyHoursWhenMetaZero 任务 hours=0 且用户不传 → 不提交 addCircle。
func TestSubmitTask_EmptyHoursWhenMetaZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			// hours=0：前端 hoursStatus=false，用户须手填
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"劳动","circle_type_id":1,"hours":0,"type_name":"劳动","dimension_id":1,"dimension_name":"劳动素养","task_id":2002,"remark":"","type":1}}`))
		case "/api/studentCircleNew/addCircle":
			t.Error("meta.hours=0 且用户空 Hours 不应到达 addCircle")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL), WithTimeout(5*1000*1000*1000))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.SubmitTask(context.Background(), "tok", types.TaskSubmitInput{
		TaskID:  2002,
		Content: "劳动内容",
		// Hours 留空
	})
	if err == nil {
		t.Fatal("任务无预设学时且未填 Hours 应失败")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("应 errors.Is(ErrInvalidPayload)，实际: %v", err)
	}
}

// TestSubmitTask_EmptyHoursUsesMeta 任务 hours>0 且用户空 → 请求体 hours=任务元数据。
func TestSubmitTask_EmptyHoursUsesMeta(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"班会","circle_type_id":9256,"hours":1.5,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"","type":10}}`))
		case "/api/studentCircleNew/addCircle":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL), WithTimeout(5*1000*1000*1000))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.SubmitTask(context.Background(), "tok", types.TaskSubmitInput{
		TaskID:  1001,
		Content: "班会内容",
	})
	if err != nil {
		t.Fatalf("预设学时任务空 Hours 应成功: %v", err)
	}
	h, ok := gotBody["hours"]
	if !ok {
		t.Fatal("请求体应含 hours")
	}
	if h != float64(1.5) {
		t.Errorf("期望 hours=1.5（任务元数据），实际 %v (%T)", h, h)
	}
}

// TestSubmitTask_NoInventedDefaults 空 Address/OrgName/Level 不对齐前端——前端不自动填学校名/等级 5。
// SDK 不得再发明默认值；用户未填则请求体为空串。
func TestSubmitTask_NoInventedDefaults(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			// 即使仍调用 GetMyInfo，schoolName 也不得写入 address/orgName
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","schoolName":"测试中学"}}`))
		case "/api/studentCircleNew/getCircleTypeByTaskId":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","dataMap":{"task_name":"班会","circle_type_id":9256,"hours":1.0,"type_name":"主题班会","dimension_id":9,"dimension_name":"思想品德","task_id":1001,"remark":"","type":10}}`))
		case "/api/studentCircleNew/addCircle":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"提交成功"}`))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL), WithTimeout(5*1000*1000*1000))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.SubmitTask(context.Background(), "tok", types.TaskSubmitInput{
		TaskID:  1001,
		Content: "不发明默认",
		// Address / OrgName / Level 全部留空
	})
	if err != nil {
		t.Fatalf("SubmitTask 应成功: %v", err)
	}
	if gotBody["address"] != "" && gotBody["address"] != nil {
		t.Errorf("address 应为空串，不得填 schoolName，实际 %v", gotBody["address"])
	}
	if gotBody["orgName"] != "" && gotBody["orgName"] != nil {
		t.Errorf("orgName 应为空串，不得填 schoolName，实际 %v", gotBody["orgName"])
	}
	if gotBody["level"] != "" && gotBody["level"] != nil {
		t.Errorf("level 应为空串，不得默认 \"5\"，实际 %v", gotBody["level"])
	}
	// 显式非空仍透传
	gotBody = nil
	_, err = c.SubmitTask(context.Background(), "tok", types.TaskSubmitInput{
		TaskID:  1001,
		Content: "显式字段",
		Address: "操场",
		OrgName: "团委",
		Level:   "3",
	})
	if err != nil {
		t.Fatalf("显式字段 SubmitTask 失败: %v", err)
	}
	if gotBody["address"] != "操场" {
		t.Errorf("address 期望 操场，实际 %v", gotBody["address"])
	}
	if gotBody["orgName"] != "团委" {
		t.Errorf("orgName 期望 团委，实际 %v", gotBody["orgName"])
	}
	if gotBody["level"] != "3" {
		t.Errorf("level 期望 3，实际 %v", gotBody["level"])
	}
}
