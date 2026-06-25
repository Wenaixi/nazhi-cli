// Package client_test 包含 review-tdd round-4 group-D Finding F 的测试：
// 验证 FetchTasks 在单维度返回业务错误（code != 1）时不再静默吞咽，
// 而是通过 errgroup propagate 出带 ErrBusinessRejected 信号的整体错误。
//
// 保留语义：
//   - 成功的维度任务仍聚合到返回的 []types.Task（不被失败维度连累）
//   - 错误用 ErrBusinessRejected 包装，errors.Is 命中
//   - 错误信息包含失败维度的统计（>0 时），方便 SDK 用户排障
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
)

// TestFindingF_FetchTasks_BizErrorPropagates 验证 FetchTasks 在某维度返回
// 业务错误时，错误不被静默吞咽，而是 propagate 为带 ErrBusinessRejected
// 信号的 error。
//
// 关键：原实现用 logDebug+return nil，单维度业务错误完全丢失。
// 修复后必须：
//  1. errors.Is(err, client.ErrBusinessRejected) = true
//  2. 成功维度的任务仍聚合到返回切片（不 fail-fast 全盘丢失）
//  3. errors.Is(err, client.ErrLoginRejected) = false（防止误判为登录问题）
func TestFindingF_FetchTasks_BizErrorPropagates(t *testing.T) {
	const dimCount = 3
	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B-业务错误"}, // 这个会返回 code != 1
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
				// 业务错误：code != 1
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

	// 断言 1：必须返回 error（旧实现会返回 nil 静默吞掉）
	if err == nil {
		t.Fatal("FetchTasks 单维度业务错误应 propagate，不应被静默吞咽")
	}

	// 断言 2：必须是 ErrBusinessRejected 包装
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("FetchTasks 业务错误应被包装为 ErrBusinessRejected，err=%v", err)
	}

	// 断言 3：不应被误判为 ErrLoginRejected
	if errors.Is(err, client.ErrLoginRejected) {
		t.Errorf("FetchTasks 业务错误不应被误判为 ErrLoginRejected，err=%v", err)
	}

	// 断言 4：成功维度的任务仍聚合（保留"单维度失败不影响其他维度"语义）
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
	// 错误信息应包含失败维度的 id 或 name，方便定位
	if !strings.Contains(errStr, "2") && !strings.Contains(errStr, "失败B-业务错误") {
		t.Errorf("错误信息应包含失败维度标识，err=%v", errStr)
	}
	// 应包含业务错误的 msg
	if !strings.Contains(errStr, "维度无数据") {
		t.Errorf("错误信息应包含业务错误 msg，err=%v", errStr)
	}
}

// TestFindingF_FetchTasks_HTTPErrorStillLogDebug 验证 F 修复**没有过度泛化**：
// 单维度 HTTP 网络错误（如 500 连接错）仍走 logDebug，不影响整体拉取
// （这是与 finding E 业务错误的关键区别：网络抖动不该 fail-fast）。
func TestFindingF_FetchTasks_HTTPErrorStillLogDebug(t *testing.T) {
	const dimCount = 3
	dims := []map[string]any{
		{"id": 1, "name": "成功A"},
		{"id": 2, "name": "失败B-HTTP500"}, // HTTP 500
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
	// HTTP 错误（非业务 code 错误）应仍走 logDebug，不 fail-fast
	if err != nil {
		t.Errorf("HTTP 错误仍应走 best-effort 模式（不 fail-fast），err=%v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("期望 2 个成功任务（维度 1 + 3），得到 %d", len(tasks))
	}
	if loggerCalls.Load() == 0 {
		t.Error("HTTP 错误仍应触发 logDebug")
	}
}
