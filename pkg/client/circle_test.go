// circle_test.go 写实评论/点赞/类别等 SDK 测试。
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestGetCircleTypes_QueryEscapesPid 验证 pid 经 url.QueryEscape 后放入查询串，
// 避免 &/= 等字符破坏 query 解析。
func TestGetCircleTypes_QueryEscapesPid(t *testing.T) {
	// 含 & 与 = 的 pid：未转义时服务端会把 query 拆碎
	const rawPID = "a&b=c"
	var gotRawQuery string
	var gotPID string
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getCircleType" {
			gotRawQuery = r.URL.RawQuery
			gotPID = r.URL.Query().Get("pid")
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"dataList": []map[string]any{},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	_, err := c.GetCircleTypes(context.Background(), "test-token", 1001, rawPID)
	if err != nil {
		t.Fatalf("GetCircleTypes 失败: %v", err)
	}
	if gotRawQuery == "" {
		t.Fatal("未收到 getCircleType 请求")
	}
	// 解析后的 pid 必须完整还原（修复后才成立；未转义时为 "a"）
	if gotPID != rawPID {
		t.Errorf("期望 Query().Get(pid)=%q，实际 %q（RawQuery=%q）", rawPID, gotPID, gotRawQuery)
	}
	wantEscaped := url.QueryEscape(rawPID)
	if !strings.Contains(gotRawQuery, "pid="+wantEscaped) {
		t.Errorf("期望 RawQuery 含 pid=%s，实际 RawQuery=%q", wantEscaped, gotRawQuery)
	}
}
