// Package client_test 包含 FetchTasks 并发拉取维度的针对性测试。
package client_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestFetchTasks_Parallel 验证 FetchTasks 并发拉取多维度任务。
// 场景：mock server 返回 5 个维度，每个维度的 getCircleStatistics
// 故意 sleep 100ms 模拟网络/服务端耗时。
// 性能断言：5 维度 × 100ms 串行 = 500ms；并发拉取应 < 250ms。
// 串行版本会被此断言捕获，并发版本通过。
// 并发安全断言：通过原子计数器验证 5 个 getCircleStatistics 全部
// 被请求（无丢失、无重复）。
func TestFetchTasks_Parallel(t *testing.T) {
	const dimCount = 5
	const perDimDelay = 100 * time.Millisecond

	// 5 个维度（id=0 会被 FetchTasks 跳过，所以 id 从 1 开始）
	dims := make([]map[string]any, 0, dimCount)
	for i := 1; i <= dimCount; i++ {
		dims = append(dims, map[string]any{
			"id":   int64(i * 10),
			"name": "维度" + string(rune('A'+i-1)),
		})
	}

	var statCalls atomic.Int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			statCalls.Add(1)
			// 模拟服务端处理耗时
			time.Sleep(perDimDelay)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 1000 + statCalls.Load(), "name": "任务", "circleTypeId": 9999, "hours": 1.0, "circleTaskStatus": "未提交", "upPic": 1},
			})))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)

	start := time.Now()
	tasks, err := c.FetchTasks(context.Background(), "test-token")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("FetchTasks 失败: %v", err)
	}

	// 性能断言：5 维度并发应明显快于 5*100ms
	const serialBound = 5 * perDimDelay // 500ms
	const parallelBound = 250 * time.Millisecond
	if elapsed >= serialBound {
		t.Errorf("FetchTasks 耗时 %v 接近串行上限 %v（并发未生效？）", elapsed, serialBound)
	}
	if elapsed >= parallelBound {
		t.Errorf("FetchTasks 耗时 %v 超过并发期望 %v", elapsed, parallelBound)
	}
	t.Logf("FetchTasks 拉取 %d 维度耗时 %v（串行期望 500ms）", dimCount, elapsed)

	// 并发安全断言：每个维度的 stat 都应被请求恰好一次
	if got := statCalls.Load(); got != dimCount {
		t.Errorf("getCircleStatistics 应被调用 %d 次, 得到 %d", dimCount, got)
	}

	// 业务断言：5 个维度各返回 1 个任务
	if len(tasks) != dimCount {
		t.Errorf("期望 %d 个任务, 得到 %d", dimCount, len(tasks))
	}

	// 维度名称应正确注入
	for _, task := range tasks {
		if task.DimensionName == "" {
			t.Errorf("任务 %d 缺少 DimensionName 注入", task.ID)
		}
	}
}

// TestFetchTasks_PartialFailure 验证单个维度失败不影响其他维度。
// 行为契约：原代码用 logDebug 不中断，重构后必须保留此行为。
func TestFetchTasks_PartialFailure(t *testing.T) {
	const dimCount = 3

	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B"}, // 这个会 HTTP 500
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
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 2000, "name": "任务" + dimID, "circleTypeId": 9999, "hours": 1.0, "circleTaskStatus": "未提交", "upPic": 1},
			})))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	// 注入 logger 捕获 logDebug 输出
	var loggerCalls atomic.Int32
	c := newTestClient(nil, biz, nil)
	_ = c

	cWithLogger, _ := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithLogger(slog.New(slog.NewTextHandler(testLogWriter{onWrite: func() {
			loggerCalls.Add(1)
		}}, &slog.HandlerOptions{Level: slog.LevelDebug}))),
	)

	tasks, err := cWithLogger.FetchTasks(context.Background(), "test-token")
	// 后：网络/解析错误不再静默吞咽，而是通过 dimErrs 聚合为 ErrBusinessRejected。
	// 因此 FetchTasks 返回 (部分成功数据, ErrBusinessRejected)，err 不为 nil。
	if err == nil {
		t.Fatal("FetchTasks 应因单维度部分失败返回 ErrBusinessRejected")
	}
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("期望错误包装 ErrBusinessRejected, 得到: %v", err)
	}
	// 期望：成功维度 2 个 × 1 任务 = 2 个任务
	if len(tasks) != 2 {
		t.Errorf("期望 2 个任务（成功维度各 1 个），得到 %d", len(tasks))
	}
	if loggerCalls.Load() == 0 {
		t.Error("期望 logger 至少被调用 1 次（记录失败维度）")
	}
}

// testLogWriter 桥接 slog 到原子计数器，仅用于测试。
type testLogWriter struct {
	onWrite func()
}

func (w testLogWriter) Write(p []byte) (int, error) {
	if w.onWrite != nil {
		w.onWrite()
	}
	return len(p), nil
}
