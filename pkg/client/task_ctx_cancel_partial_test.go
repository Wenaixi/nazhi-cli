// r9-D1 修复测试：验证 context 取消后 FetchTasks 在部分任务已获取时
// 返回 (partialTasks, error) 且 errors.Is(ErrBusinessRejected) 为 true。
//
// 设计动机：当部分维度在 context 取消前已完成（len(allTasks) > 0），
// 其余维度因 cancel 产生 dimErrs 时，返回的错误必须包装 ErrBusinessRejected，
// 让 cmd 层 envelope 分支能正确识别并输出 partial 状态。
//
// 与 B11 设计的边界：
//   - 无 partial tasks（allTasks 为空）→ 裸 context.Canceled（不包装）
//   - 有 partial tasks（allTasks 非空）→ 包装 ErrBusinessRejected
//
// 建立信任的 3 个断言：
//  1. errors.Is(err, context.DeadlineExceeded) == true（保留 ctx 根因）
//  2. errors.Is(err, ErrBusinessRejected) == true（cmd 层 envelope 可识别）
//  3. len(tasks) > 0（partial tasks 已保留）
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

// TestFetchTasks_ContextCancel_ReturnsErrBusinessRejected 验证 context 取消后
// FetchTasks 在部分维度已成功时返回 errors.Is(ErrBusinessRejected) 且包含 partial tasks。
//
// 设计：4 个维度（id=10/20/30/40），errgroup.SetLimit(4) 不会限制任何 goroutine，
// 所有维度并发启动。用 dim.ID 决定行为避免计数 race：
//   - dim.ID <= 20（维度A/B）：handler 立即返回（在 context 取消前完成）
//   - dim.ID > 20（维度C/D）：handler 睡眠 1s → context 超时 → 返回 DeadlineExceeded
//
// 前 2 个维度在 context 200ms 超时前完成（partial tasks），后 2 个因超时失败。
//
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
				time.Sleep(time.Second)
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
