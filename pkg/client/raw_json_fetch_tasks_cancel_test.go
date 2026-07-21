// FetchTasksJSON cancel 路径：必须透出 ErrRetryable（与 FetchTasks 对称）。
package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestFetchTasksJSON_ContextCancel_HitsErrRetryable 锁定：
// 维度 g.Go 在 ctx 取消时必须能让 errors.Is(err, ErrRetryable) 命中。
// 旧实现 g.Go 永不 return err，ctx cancel 被吞进 dimErrs 后统一包成
// ErrBusinessRejected，丢失可重试语义。
func TestFetchTasksJSON_ContextCancel_HitsErrRetryable(t *testing.T) {
	var statsHits int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			body := map[string]any{
				"code": 1,
				"dataList": []map[string]any{
					{"id": 1, "name": "维度A"},
					{"id": 2, "name": "维度B"},
					{"id": 3, "name": "维度C"},
				},
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/api/studentCircleNew/getCircleStatistics":
			atomic.AddInt32(&statsHits, 1)
			// 阻塞直到请求 ctx 取消，模拟慢维度
			select {
			case <-r.Context().Done():
				// 客户端断开，不写响应
			case <-time.After(2 * time.Second):
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code":     1,
					"dataList": []map[string]any{{"id": 100, "name": "慢任务"}},
				})
			}
		default:
			http.NotFound(w, r)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	// 短超时：getDimensions 可完成，但 getCircleStatistics 被 cancel 截断
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	raw, err := c.FetchTasksJSON(ctx, "test-token")
	if err == nil {
		t.Fatal("ctx 取消应返回 error")
	}
	if !errors.Is(err, client.ErrRetryable) {
		t.Errorf("错误链应含 ErrRetryable，得到 %v", err)
	}
	// 允许 partial 字节（有成功维度时）或 nil（全 cancel）
	if len(raw) > 0 {
		var probe any
		if jerr := json.Unmarshal(raw, &probe); jerr != nil {
			t.Errorf("partial 字节仍须合法 JSON: body=%s err=%v", raw, jerr)
		}
	}
}

// TestFetchTasksJSON_PartialSuccess_CancelStillErrRetryable 有部分维度成功时：
// 仍应 errors.Is(ErrRetryable)，并附带已合并字节。
func TestFetchTasksJSON_PartialSuccess_CancelStillErrRetryable(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			body := map[string]any{
				"code": 1,
				"dataList": []map[string]any{
					{"id": 1, "name": "快维度"},
					{"id": 2, "name": "慢维度"},
				},
			}
			_ = json.NewEncoder(w).Encode(body)
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			w.Header().Set("Content-Type", "application/json")
			if dimID == "1" {
				// 快维度立即返回
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code": 1,
					"dataList": []map[string]any{
						{"id": 1001, "name": "任务快", "rawExtra": "keep"},
					},
				})
				return
			}
			// 慢维度阻塞
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
				_ = json.NewEncoder(w).Encode(map[string]any{
					"code":     1,
					"dataList": []map[string]any{{"id": 2001}},
				})
			}
		default:
			http.NotFound(w, r)
		}
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	raw, err := c.FetchTasksJSON(ctx, "test-token")
	if err == nil {
		t.Fatal("ctx 取消应返回 error")
	}
	if !errors.Is(err, client.ErrRetryable) {
		t.Errorf("部分成功 + cancel 仍应含 ErrRetryable，得到 %v", err)
	}
	// 有 partial 时 cmd 层还要 ErrBusinessRejected 识别 partial envelope
	if !errors.Is(err, client.ErrBusinessRejected) {
		t.Errorf("有 partial 数据时应包装 ErrBusinessRejected，得到 %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("应返回已合并部分字节")
	}
	if !containsRaw(string(raw), `"rawExtra":"keep"`) {
		t.Errorf("应保留成功维度 raw 字段, body=%s", raw)
	}
}

func containsRaw(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
