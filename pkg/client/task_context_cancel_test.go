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

// TestFetchTasks_ContextCancel_PropagatesError 验证 context 取消后 FetchTasks
// 返回的错误包含 context.Canceled/DeadlineExceeded，而非被静默吞掉后
// 包装为 ErrBusinessRejected。
//
// 设计：10 个维度超过 errgroup.SetLimit(8) 的并发上限，其中 2 个会排队。
// 每个维度的 handler 睡眠 200ms，而 context 只有 5ms 超时。
// 这样：
//   - 8 个 goroutine 立即启动 → handler 睡眠 → 5ms 后 ctx 超时
//   - HTTP transport 取消 8 个 in-flight 请求 → doRequest 返回 DeadlineExceeded
//   - 8 个 goroutine 的闭包返回 nil（旧行为，dimErr 被吞）→ 释放 semaphore slot
//   - 2 个排队的 goroutine 启动
//   - WITH fix: 闭包顶部检查 gctx.Err() → 直接返回 DeadlineExceeded 给 errgroup
//   - WITHOUT fix: 不检查 → fetchTasksForDimension 返回 (nil, DeadlineExceeded)
//     → 闭包吞掉 → return nil → g.Wait() 返回 nil → 包装为 ErrBusinessRejected
//
// 关键断言：errors.Is(err, context.DeadlineExceeded) 为 true，
// 且 errors.Is(err, client.ErrBusinessRejected) 为 false。
func TestFetchTasks_ContextCancel_PropagatesError(t *testing.T) {
	// 10 个维度，超过 SetLimit(8)，确保有排队 goroutine
	dims := make([]map[string]any, 10)
	for i := range dims {
		dims[i] = map[string]any{
			"id":   int64((i + 1) * 10),
			"name": "维度" + string(rune('A'+i)),
		}
	}

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			// 每个 handler 睡眠足够久，确保 context 先超时
			time.Sleep(200 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 1000, "name": "任务", "circleTypeId": 9999, "hours": 1.0, "circleTaskStatus": "未提交", "upPic": 1},
			})))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	// 5ms 超时，远小于 handler 的 200ms 睡眠
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := c.FetchTasks(ctx, "test-token")
	if err == nil {
		t.Fatal("FetchTasks 应在 context 超时时返回错误")
	}

	// 不应是 ErrBusinessRejected（旧行为：cancel 被 dimErrs 吞掉后包装为此）
	if errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("期望 context 超时错误而非 ErrBusinessRejected, 得到: %v", err)
	}

	// 应包含 context.DeadlineExceeded
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("期望错误包含 context.DeadlineExceeded, 得到: %v", err)
	}
}
