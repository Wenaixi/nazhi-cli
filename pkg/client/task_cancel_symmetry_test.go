// Package client 白盒测试：F2.2 cancel 路径对称性。
//
// 修复动机：task.go 的 cancel 路径有 2 个分支：
//   - 纯 cancel 路径（bizErrs 空 + cancelledCount > 0）：errors.Join 含 cancelPlaceholder
//   - 混合路径（bizErrs 非空 + cancelledCount > 0）：errors.Join 含 cancelPlaceholder，再包装 ErrBusinessRejected
//
// F2.1 改 cancelPlaceholder = fmt.Errorf("%w: ...", ErrRetryable, ...) 后，
// 两路径走 errors.Join 都会让最终 error 链上含 ErrRetryable。
//
// 本测试锁死「两路径都 errors.Is(err, ErrRetryable) 命中」的不变量，
// 防止未来重构破坏 cancel 路径语义对称性。
//
// ponytail：测试在 internal package（client），自建 httptest handler 和 Client，
// 不依赖外部测试包的 warmupBizHandler/unifiedJSON/newTestClient helpers
// （避免 import cycle 风险 + 让本测试自包含）。
package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// f22Client 构造一个最小可用 Client（自包含测试，不依赖外部 helper）。
func f22Client(bizURL string) *Client {
	return &Client{
		ssoBaseURL: bizURL,
		baseURL:    bizURL,
		uploadURL:  bizURL,
		http:       newHTTPClient(),
		logger:     nil,
		ocr:        nil,
		sm:         &sessionManager{},
	}
}

// f22BizHandler 构造一个 mock biz server，支持 dims 配置 + 阻塞模式。
//
// dimsJSON：返回给 /getDimensions 的 dims JSON
// dimIDToBizErr：哪些 dimID 立即返回业务错误（code=0）
// session 激活走 4 步契约（/, /getMenu x2, /getMyInfo），都需要正确响应。
func f22BizHandler(t *testing.T, dimsJSON string, dimIDToBizErr map[string]bool) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html>home</html>`))
		case "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// returnData 含真实 UserInfo，避免 getMyInfoRaw 返回 ErrEmptyUserInfo
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"id":1,"name":"test"}}`))
		case "/api/studentCircleNew/getDimensions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(dimsJSON))
		case "/api/studentCircleNew/getCircleStatistics":
			dimID := r.URL.Query().Get("dimensionId")
			if dimIDToBizErr[dimID] {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"code":0,"msg":"业务失败","returnData":null}`))
				return
			}
			// 阻塞直到 ctx 取消（net/http server 收到 cancel 后断开连接）
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("未预期的请求: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

// TestF22_PureCancel_HitsErrRetryable 纯 cancel 路径（无业务错误）：
// errors.Is(err, ErrRetryable) 必须为 true。
func TestF22_PureCancel_HitsErrRetryable(t *testing.T) {
	dims := []map[string]any{
		{"id": int64(10), "name": "维度A"},
		{"id": int64(20), "name": "维度B"},
		{"id": int64(30), "name": "维度C"},
	}
	dimsJSON, _ := json.Marshal(types.UnifiedResponse{
		Code: 1, Msg: ptrStr("成功"),
		DataList: mustMarshal(t, dims),
	})

	biz := httptest.NewServer(f22BizHandler(t, string(dimsJSON), nil))
	defer biz.Close()

	c := f22Client(biz.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	tasks, err := c.FetchTasks(ctx, "test-token")
	if err == nil {
		t.Fatal("纯 cancel 应返回非 nil error")
	}
	if len(tasks) != 0 {
		t.Errorf("纯 cancel 路径 allTasks 应为空，实际 %d", len(tasks))
	}
	if !errors.Is(err, ErrRetryable) {
		t.Errorf("纯 cancel 路径 errors.Is(err, ErrRetryable) 应为 true，实际: %v", err)
	}
	if errors.Is(err, ErrBusinessRejected) {
		t.Errorf("纯 cancel 路径不应包装 ErrBusinessRejected，实际: %v", err)
	}
	t.Logf("纯 cancel 路径 err=%v (ErrRetryable 命中 ✓)", err)
}

// TestF22_MixedCancelAndBiz_HitsErrRetryable 混合路径（业务错误 + ctx cancel）：
// errors.Is(err, ErrRetryable) 必须为 true（cancelPlaceholder 仍 in chain）。
func TestF22_MixedCancelAndBiz_HitsErrRetryable(t *testing.T) {
	dims := []map[string]any{
		{"id": int64(10), "name": "维度A"},
		{"id": int64(20), "name": "维度B"},
		{"id": int64(30), "name": "维度C"},
	}
	dimsJSON, _ := json.Marshal(types.UnifiedResponse{
		Code: 1, Msg: ptrStr("成功"),
		DataList: mustMarshal(t, dims),
	})

	biz := httptest.NewServer(f22BizHandler(t, string(dimsJSON), map[string]bool{
		"10": true, // 维度A 立即返回业务错误
		// 维度 B/C 阻塞 → ctx 取消
	}))
	defer biz.Close()

	c := f22Client(biz.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.FetchTasks(ctx, "test-token")
	if err == nil {
		t.Fatal("混合路径应返回非 nil error")
	}
	if !errors.Is(err, ErrRetryable) {
		t.Errorf("混合路径 errors.Is(err, ErrRetryable) 应为 true，实际: %v", err)
	}
	if !errors.Is(err, ErrBusinessRejected) {
		t.Errorf("混合路径 errors.Is(err, ErrBusinessRejected) 应为 true（包装层），实际: %v", err)
	}
	t.Logf("混合路径 err=%v (ErrRetryable + ErrBusinessRejected 双命中 ✓)", err)
}

// TestF22_PureBizError_DoesNotHitErrRetryable 纯业务错误路径（无 cancel）：
// errors.Is(err, ErrRetryable) 必须为 false（语义对称：cancel sentinel 仅在 cancel 触发时命中）。
func TestF22_PureBizError_DoesNotHitErrRetryable(t *testing.T) {
	dims := []map[string]any{
		{"id": int64(10), "name": "维度A"},
	}
	dimsJSON, _ := json.Marshal(types.UnifiedResponse{
		Code: 1, Msg: ptrStr("成功"),
		DataList: mustMarshal(t, dims),
	})

	biz := httptest.NewServer(f22BizHandler(t, string(dimsJSON), map[string]bool{
		"10": true,
	}))
	defer biz.Close()

	c := f22Client(biz.URL)
	tasks, err := c.FetchTasks(context.Background(), "test-token")
	t.Logf("DEBUG test3: tasks=%v err=%v IsBizRej=%v IsRetry=%v", tasks, err, errors.Is(err, ErrBusinessRejected), errors.Is(err, ErrRetryable))
	t.Logf("DEBUG test3: biz handler should have been hit at / and /getMenu etc.")
	if err == nil {
		t.Fatal("纯业务错误应返回非 nil error")
	}
	if errors.Is(err, ErrRetryable) {
		t.Errorf("纯业务错误不应命中 ErrRetryable，实际: %v", err)
	}
	if !errors.Is(err, ErrBusinessRejected) {
		t.Errorf("纯业务错误应命中 ErrBusinessRejected，实际: %v", err)
	}
}

// ptrStr 返回字符串指针（types.DerefOr 配对使用）。
func ptrStr(s string) *string { return &s }

// mustMarshal 序列化为 *json.RawMessage（匹配 types.UnifiedResponse.ReturnData 类型）。
func mustMarshal(t *testing.T, v any) *json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	rm := json.RawMessage(b)
	return &rm
}
