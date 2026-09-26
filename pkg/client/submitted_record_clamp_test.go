package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestGetSubmittedCircles_TotalNumClampedByRecordUpperBound 锁定
// 服务端声明 totalNum 超过条数上界 maxSubmittedRecords（10 万）时，
// 翻页合并必须直接截断返回首页，不得按 totalNum 预分配 GB 级切片。
//
// 旧实现只做页数钳制（maxTotalPage*pageSize=5M 条，每条约 1KB → 单次
// make 预分配 4.5-5GB）；WithSubmittedPageSize 调大 pageSize（如 10 万）
// 还能让钳制膨胀到 1e9 条 → 直接 OOM。修复后按条数上界二次钳制。
//
// 病态响应（服务端同一响应 totalNum 超上界、totalPage 虚高 5001）的
// 断言只验证「截断不翻页、保留首页」，与新上界 maxSubmittedRecords
// 强绑定；若未来有意提高上界，本测试的 500001 构造需同步放大。
func TestGetSubmittedCircles_TotalNumClampedByRecordUpperBound(t *testing.T) {
	var callCount int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			// 翻页请求会携带 pageNo query，但与首页共享同一 path——
			// 旧实现不按条数上界截断时会反复命中此分支并不断翻页
			callCount++
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code": 1,
				// totalNum=500001 超过 10 万条上界，首页只返回 1 条
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 500001, 5001)),
				"dataList": []map[string]any{submittedRecord(1, "首页", 0)},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithSubmittedPageSize(submittedPageSize),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	// 不能 panic、不能 OOM、不翻页；应返回首页快照（与 totalPage 超钳制分支一致）
	records, err := c.GetSubmittedCircles(context.Background(), "tok", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("totalNum 超条数上界后不应翻页，实际 API 调用 %d 次", callCount)
	}
	if len(records) != 1 || records[0].ID != 1 {
		t.Fatalf("超限截断应只返回首页记录，实际条数=%d 首条ID=%d", len(records), records[0].ID)
	}
}
