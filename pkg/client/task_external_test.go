// task_external_test.go 聚合 task.go 外部黑盒测试（package client_test）：
//   - F2: FetchTasks errgroup.SetLimit(8) 并发上限
//   - F12 PLAUSIBLE: FetchTasks 并发拉取维度（性能 + 计数）
//   - F4: 部分维度失败时聚合 partial failures
//   - ctx: context 取消 propagate（无 partial / 有 partial 两种路径）
//   - GetDimensions: 维度列表 + 业务错误
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

// ─── task_concurrent_limit_test.go (F2): FetchTasks 并发上限 ───

// TestFetchTasks_ConcurrentLimitBounded 回归测试 F2：FetchTasks 的维度并发拉取
// 必须有上限（errgroup.SetLimit），避免 N=50+ 维度的业务场景爆掉服务端。
// 测试策略：
// - mock server 返回 20 个维度（id=1..20，id=0 会跳过）
// - 每个 getCircleStatistics sleep 50ms（足够长让并发 goroutine 同时在飞）
// - 服务端用 atomic 跟踪当前正在处理的请求数（in-flight），记录峰值
// - 断言峰值 ≤ 预期上限（min(20, 8) = 8）
// 历史 bug（F12 PLAUSIBLE）：原实现对每个 dimension 起一个 goroutine，TODO 注释
// 提到"如未来接入 > 50 维度需引入 semaphore"但没真做。F2 修复落实 semaphore，
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

// ─── task_concurrent_test.go (F12): FetchTasks 并发拉取维度 ───

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

// ─── task_context_cancel_test.go: ctx 取消 propagate（无 partial） ───

// TestFetchTasks_ContextCancel_PropagatesError 验证 context 取消后 FetchTasks
// 返回的错误包含 context.Canceled/DeadlineExceeded，而非被静默吞掉后
// 包装为 ErrBusinessRejected。
// 设计：10 个维度超过 errgroup.SetLimit(8) 的并发上限，其中 2 个会排队。
// 每个维度的 handler 睡眠 200ms，而 context 只有 5ms 超时。
// 这样：
// - 8 个 goroutine 立即启动 → handler 睡眠 → 5ms 后 ctx 超时
// - HTTP transport 取消 8 个 in-flight 请求 → doRequest 返回 DeadlineExceeded
// - 8 个 goroutine 的闭包返回 nil（旧行为，dimErr 被吞）→ 释放 semaphore slot
// - 2 个排队的 goroutine 启动
// - WITH fix: 闭包顶部检查 gctx.Err() → 直接返回 DeadlineExceeded 给 errgroup
// - WITHOUT fix: 不检查 → fetchTasksForDimension 返回 (nil, DeadlineExceeded)
// → 闭包吞掉 → return nil → g.Wait() 返回 nil → 包装为 ErrBusinessRejected
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
			time.Sleep(50 * time.Millisecond)
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

// ─── task_ctx_cancel_partial_test.go (r9-D1): ctx 取消 + partial 包装 ───

// TestFetchTasks_ContextCancel_ReturnsErrBusinessRejected 验证 context 取消后
// FetchTasks 在部分维度已成功时返回 errors.Is(ErrBusinessRejected) 且包含 partial tasks。
// 设计：4 个维度（id=10/20/30/40），errgroup.SetLimit(4) 不会限制任何 goroutine，
// 所有维度并发启动。用 dim.ID 决定行为避免计数 race：
// - dim.ID <= 20（维度A/B）：handler 立即返回（在 context 取消前完成）
// - dim.ID > 20（维度C/D）：handler 睡眠 1s → context 超时 → 返回 DeadlineExceeded
// 前 2 个维度在 context 200ms 超时前完成（partial tasks），后 2 个因超时失败。
// RED 阶段：期望红色（旧行为返回裸 context 错误，ErrBusinessRejected 不在链上）
// GREEN 阶段：errors.Is(err, ErrBusinessRejected) 为 true
func TestFetchTasks_ContextCancel_ReturnsErrBusinessRejected(t *testing.T) {
	dims := []map[string]any{
		{"id": int64(10), "name": "维度A"},
		{"id": int64(20), "name": "维度B"},
		{"id": int64(30), "name": "维度C"},
		{"id": int64(40), "name": "维度D"},
	}

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, dims)))
		case "/api/studentCircleNew/getCircleStatistics":
			// 用 dimensionId 参数决定是否 sleep，避免并发 handler 的计数 race
			dimID := r.URL.Query().Get("dimensionId")
			if dimID == "30" || dimID == "40" {
				// 后 2 个维度睡眠足够久，确保 context 先超时
				time.Sleep(300 * time.Millisecond)
			}
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

	// 200ms 超时：足够 session 激活（4步即时响应） + getDimensions + 前 2 个 getCircleStatistics，
	// 后 2 个 getCircleStatistics（sleep 1s）会超时
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	tasks, err := c.FetchTasks(ctx, "test-token")
	if err == nil {
		t.Fatal("FetchTasks 应在 context 超时时返回错误")
	}

	// 断言 1：保留 context 根因（调用方应能识别 context 取消）
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("期望 errors.Is(err, context.DeadlineExceeded), 得到: %v", err)
	}

	// 断言 2：必须包装 ErrBusinessRejected（cmd 层 envelope 识别）
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("期望 errors.Is(err, ErrBusinessRejected)（含 partial tasks 时应包装）, 得到: %v", err)
	}

	// 断言 3：前 2 个维度的任务应被保留
	if len(tasks) == 0 {
		t.Error("期望有 partial tasks 返回（前 2 个维度应成功）")
	}
}

// ─── task_dimensions_test.go: GetDimensions ───

// TestGetDimensions 验证 GetDimensions 返回完整维度列表。
// 用途：Finding #6 抽取 fetchDimensions helper 后此测试继续通过，
// 证明 helper 行为与原 GetDimensions 等价。
func TestGetDimensions(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedJSON(1, "成功", nil, []map[string]any{
				{"id": 0, "name": "全部"},
				{"id": 9, "name": "思想品德"},
				{"id": 14, "name": "劳动素养"},
			})))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	dims, err := c.GetDimensions(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetDimensions 失败: %v", err)
	}
	if len(dims) != 3 {
		t.Fatalf("期望 3 个维度, 得到 %d", len(dims))
	}
	if dims[0].ID != 0 || dims[0].Name != "全部" {
		t.Errorf("维度 0 错误: %+v", dims[0])
	}
	if dims[1].ID != 9 || dims[1].Name != "思想品德" {
		t.Errorf("维度 1 错误: %+v", dims[1])
	}
	if dims[2].ID != 14 || dims[2].Name != "劳动素养" {
		t.Errorf("维度 2 错误: %+v", dims[2])
	}
}

// TestGetDimensions_BizError 验证 GetDimensions 在业务错误时返回错误。
func TestGetDimensions_BizError(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// code=0 业务错误
			_, _ = w.Write([]byte(unifiedJSON(0, "未授权", nil, nil)))
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetDimensions(context.Background(), "test-token")
	if err == nil {
		t.Fatal("期望业务错误，但得到 nil")
	}
}
