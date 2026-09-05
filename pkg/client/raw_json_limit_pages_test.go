// getCirclesLimitJSON 页范围裁剪：只请求 offset/limit 覆盖到的页，不全量翻页。
package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

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
	raw, pb, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 0, 2, "")
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

	raw, _, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 3, 2, "")
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

// TestGetSubmittedCirclesJSON_TotalNumRequiresMorePages 验证原始 JSON 路径不因 totalPage 虚低而截断。
func TestGetSubmittedCirclesJSON_TotalNumRequiresMorePages(t *testing.T) {
	const pageSize = 2
	var hits atomic.Int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		pageNo, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "dataList": []map[string]any{{"id": pageNo*10 + 1}}, "pageBean": map[string]any{"pageNo": pageNo, "pageSize": pageSize, "totalNum": 3, "totalPage": 1}})
	})))
	defer biz.Close()
	c, err := client.New(client.WithBaseURL(biz.URL), client.WithTimeout(5*time.Second), client.WithSubmittedPageSize(pageSize))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	raw, _, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "token", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || hits.Load() != 2 {
		t.Fatalf("原始 JSON 应按 totalNum 推导请求 2 页并返回 2 条，得到 hits=%d records=%d", hits.Load(), len(records))
	}
}

func TestGetCirclesLimitJSON_InconsistentTotalPage(t *testing.T) {
	for _, totalPage := range []int{0, 1} {
		t.Run(strconv.Itoa(totalPage), func(t *testing.T) {
			biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
				pn, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
				records := []map[string]any{{"id": pn*2 - 1}, {"id": pn * 2}}
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "dataList": records, "pageBean": map[string]any{"totalNum": 6, "totalPage": totalPage, "pageSize": 2}})
			})))
			defer biz.Close()
			c, err := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithSubmittedPageSize(2))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			raw, pb, err := c.GetSubmittedCirclesLimitJSON(t.Context(), "token", 2, 2, "")
			if err != nil {
				t.Fatal(err)
			}
			if string(raw) != `[{"id":3},{"id":4}]` {
				t.Fatalf("应返回偏移后的完整记录，实际 %s", raw)
			}
			if pb.TotalPage != totalPage {
				t.Fatalf("不能篡改服务端分页元数据: %+v", pb)
			}
		})
	}
}

// TestGetSubmittedCirclesJSON_InvalidDataListRejects 验证非法 dataList 不会被静默归一为空成功。
func TestGetSubmittedCirclesJSON_InvalidDataListRejects(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":{"id":1},"pageBean":{"pageSize":2,"totalNum":1,"totalPage":1}}`))
	})))
	defer biz.Close()
	c, err := client.New(client.WithBaseURL(biz.URL), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_, err = c.GetSubmittedCirclesJSON(context.Background(), "token", "")
	if err == nil || !errors.Is(err, client.ErrInvalidResponse) {
		t.Fatalf("非法 dataList 应返回 ErrInvalidResponse，实际 %v", err)
	}
}

// TestGetCirclesLimitJSON_FullModeKeepsPageBean 锁定全量模式仍返回分页元数据。
func TestGetCirclesLimitJSON_FullModeKeepsPageBean(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		pageNo, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
		w.Header().Set("Content-Type", "application/json")
		records := []map[string]any{{"id": 101}}
		if pageNo > 1 {
			records = []map[string]any{}
		}
		body := map[string]any{
			"code":     1,
			"dataList": records,
			"pageBean": map[string]any{
				"pageNo": 1, "pageSize": 500, "totalNum": 2532, "totalPage": 6,
			},
		}
		_ = json.NewEncoder(w).Encode(body)
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

	raw, page, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "test-token", 0, 0, "")
	if err != nil {
		t.Fatalf("GetSubmittedCirclesLimitJSON 全量模式失败: %v", err)
	}
	if page == nil {
		t.Fatal("全量模式应返回 PageBean")
	}
	if page.TotalNum != 2532 || page.TotalPage != 6 {
		t.Fatalf("全量模式分页元数据错误: %+v", page)
	}
	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("全量模式原始 JSON 非法: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("全量模式记录数错误: %d", len(records))
	}
}
