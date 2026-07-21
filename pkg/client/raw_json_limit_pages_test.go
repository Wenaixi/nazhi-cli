// getCirclesLimitJSON 页范围裁剪：只请求 offset/limit 覆盖到的页，不全量翻页。
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestGetCirclesLimitJSON_OnlyFetchesNeededPages 锁定：
// offset=0, limit=pageSize 时只需 page1，不得请求 page2..N。
// 旧实现会 for pageNo:=2..TotalPage 全量并发翻页再截断，浪费请求。
func TestGetCirclesLimitJSON_OnlyFetchesNeededPages(t *testing.T) {
	var pageHits [6]int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		if pageNo >= 1 && pageNo <= 5 {
			atomic.AddInt32(&pageHits[pageNo], 1)
		}
		w.Header().Set("Content-Type", "application/json")
		// 5 页共 10 条，pageSize=2
		body := map[string]any{
			"code": 1,
			"dataList": []map[string]any{
				{"id": pageNo*10 + 1},
				{"id": pageNo*10 + 2},
			},
			"pageBean": map[string]any{
				"pageNo":    pageNo,
				"pageSize":  pageSize,
				"totalNum":  10,
				"totalPage": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
		client.WithSubmittedPageSize(2),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	// 只要前 2 条（恰好一页），不应请求 page2..5
	raw, pb, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 0, 2)
	if err != nil {
		t.Fatalf("GetSubmittedCirclesLimitJSON: %v", err)
	}
	if pb == nil || pb.TotalNum != 10 {
		t.Fatalf("期望 TotalNum=10, pb=%v", pb)
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Fatalf("结果非合法 JSON: %v body=%s", jerr, raw)
	}
	if len(arr) != 2 {
		t.Fatalf("期望 2 条, 得到 %d", len(arr))
	}
	if pageHits[1] != 1 {
		t.Errorf("page1 应请求 1 次, 得到 %d", pageHits[1])
	}
	for pn := 2; pn <= 5; pn++ {
		if pageHits[pn] != 0 {
			t.Errorf("offset=0 limit=2 不应请求 page%d, hits=%d", pn, pageHits[pn])
		}
	}
}

// TestGetCirclesLimitJSON_EndPageByOffsetLimit offset 跨页时只拉到 endPage。
// offset=3, limit=2, pageSize=2 → 需要 page2(记录 2-3) 与 page3(记录 4-5) 中的切片，
// endPage = ceil((3+2)/2) = 3，不应请求 page4/page5。
func TestGetCirclesLimitJSON_EndPageByOffsetLimit(t *testing.T) {
	var pageHits [6]int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		pageNo, _ := strconv.Atoi(q.Get("pageNo"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		if pageNo >= 1 && pageNo <= 5 {
			atomic.AddInt32(&pageHits[pageNo], 1)
		}
		w.Header().Set("Content-Type", "application/json")
		// 每页 2 条：全局 index (pageNo-1)*2 与 (pageNo-1)*2+1
		body := map[string]any{
			"code": 1,
			"dataList": []map[string]any{
				{"id": (pageNo-1)*2 + 0},
				{"id": (pageNo-1)*2 + 1},
			},
			"pageBean": map[string]any{
				"pageNo":    pageNo,
				"pageSize":  pageSize,
				"totalNum":  10,
				"totalPage": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(body)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithSSOBase(biz.URL),
		client.WithUploadURL(biz.URL),
		client.WithSubmittedPageSize(2),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()

	raw, _, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 3, 2)
	if err != nil {
		t.Fatalf("GetSubmittedCirclesLimitJSON: %v", err)
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Fatalf("结果非合法 JSON: %v body=%s", jerr, raw)
	}
	if len(arr) != 2 {
		t.Fatalf("期望 2 条, 得到 %d body=%s", len(arr), raw)
	}
	// id 应为 3,4
	if id, _ := arr[0]["id"].(float64); id != 3 {
		t.Errorf("期望首条 id=3, 得到 %v", arr[0]["id"])
	}
	if id, _ := arr[1]["id"].(float64); id != 4 {
		t.Errorf("期望次条 id=4, 得到 %v", arr[1]["id"])
	}
	// page1 始终请求（拿 total）；page2/page3 需要；page4/5 不应请求
	if pageHits[1] != 1 {
		t.Errorf("page1 应请求 1 次, 得到 %d", pageHits[1])
	}
	if pageHits[2] != 1 {
		t.Errorf("page2 应请求 1 次, 得到 %d", pageHits[2])
	}
	if pageHits[3] != 1 {
		t.Errorf("page3 应请求 1 次, 得到 %d", pageHits[3])
	}
	if pageHits[4] != 0 || pageHits[5] != 0 {
		t.Errorf("不应请求 page4/5, hits4=%d hits5=%d", pageHits[4], pageHits[5])
	}
}
