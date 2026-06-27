// Package client_test 包含 FetchTasks 并发上限保护测试（F2 修复验证）。
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestFetchTasks_ConcurrentLimitBounded 回归测试 F2：FetchTasks 的维度并发拉取
// 必须有上限（errgroup.SetLimit），避免 N=50+ 维度的业务场景爆掉服务端。
// 测试策略：
// - mock server 返回 20 个维度（id=1..20，id=0 会跳过）
// - 每个 getCircleStatistics sleep 50ms（足够长让并发 goroutine 同时在飞）
// - 服务端用 atomic 跟踪当前正在处理的请求数（in-flight），记录峰值
// - 断言峰值 ≤ 预期上限（min(20, 8) = 8）
// 历史 bug（F12 PLAUSIBLE）：原实现对每个 dimension 起一个 goroutine，TODO 注释
// 提到\"如未来接入 > 50 维度需引入 semaphore\"但没真做。F2 修复落实 semaphore，
// 上限 = min(len(dimensions), 8)。
func TestFetchTasks_ConcurrentLimitBounded(t *testing.T) {
	const (
		dimCount      = 20
		perDimDelay   = 50 * time.Millisecond
		expectedLimit = 8 // 与 task.go 中 fetchTasksConcurrentLimit 常量一致
	)
	if expectedLimit >= dimCount {
		t.Fatalf("测试设计错误：expectedLimit (%d) 应小于 dimCount (%d)，否则无法验证并发上限", expectedLimit, dimCount)
	}

	// 20 个维度（id=0 会跳过，所以从 1 开始）
	dims := make([]map[string]any, 0, dimCount)
	for i := 1; i <= dimCount; i++ {
		dims = append(dims, map[string]any{
			"id":   int64(i),
			"name": "维度",
		})
	}

	var (
		inFlight     atomic.Int32
		peakInFlight atomic.Int32
	)

	// 模拟服务端耗时 + 跟踪 in-flight 峰值
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			cur := inFlight.Add(1)
			// 更新峰值（用 CAS 保证只记录最大）
			for {
				old := peakInFlight.Load()
				if cur <= old || peakInFlight.CompareAndSwap(old, cur) {
					break
				}
			}
			defer inFlight.Add(-1)
			time.Sleep(perDimDelay)
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

	if _, err := c.FetchTasks(context.Background(), "test-token"); err != nil {
		t.Fatalf("FetchTasks 失败: %v", err)
	}

	peak := int(peakInFlight.Load())
	t.Logf("FetchTasks %d 维度，峰值并发请求数 = %d（限制 = %d）", dimCount, peak, expectedLimit)
	if peak > expectedLimit {
		t.Errorf("并发峰值 %d 超过限制 %d（errgroup.SetLimit 未生效？）", peak, expectedLimit)
	}
	// 同时确认并发真发生了（峰值至少 2，否则等于串行，semaphore 限到 1 没意义）
	if peak < 2 {
		t.Errorf("并发峰值 %d 过低（<2），可能 errgroup.SetLimit(0) 或维度被串行化", peak)
	}
}
