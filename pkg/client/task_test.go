// task_test.go 聚合 task.go 内部白盒测试（package client）：
//   - G2: fetchTasksForDimensionSafe panic recover
//   - SubmitTask 业务错误分类
package client

import (
	"context"
	"encoding/json"
	"errors"
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
