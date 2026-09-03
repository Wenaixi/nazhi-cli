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

// TestGetSubmittedCircles_TotalNumLessThanFirstPageLen 回归测试：
// 服务端虚报 totalNum 且超发首页记录（count/list 短暂不一致的病态响应）时，
// 翻页合并不得因 make cap < len 而 panic。
//
// 背景（十三域审计 P2-K）：多页分支曾以服务端声明的 totalNum 作为 make 容量，
// 与第一页实际解码条数独立来源、无钳制；cap < len 直接 runtime panic
// （makeslice: cap out of range）。钳制后以 max(len(page1), totalNum) 为容量。
func TestGetSubmittedCircles_TotalNumLessThanFirstPageLen(t *testing.T) {
	// pageSize=100，totalPage=2、totalNum=101 满足进多页分支的前提；
	// 首页却超发 150 条（突破自己 LIMIT 的违约响应），cap(101) < len(150)。
	const overlong = 150
	records := make([]map[string]any, 0, overlong)
	for i := 1; i <= overlong; i++ {
		records = append(records, submittedRecord(int64(i), "超发记录", 0))
	}

	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 101, 2)),
				"dataList": records,
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle&pageNo=2" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(2, submittedPageSize, 101, 2)),
				"dataList": []map[string]any{submittedRecord(9999, "第二页", 0)},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithSubmittedPageSize(submittedPageSize),
	)
	if err != nil {
		t.Fatalf("构造 Client: %v", err)
	}
	defer c.Close()
	// 只要不 panic（makeslice）即通过；合并结果条数不作严格断言——
	// 病态响应下的合并语义超出本测试目标。
	_, _ = c.GetSubmittedCircles(context.Background(), "test-token", "")
}

// TestGetSubmittedCircles_TotalPageClamped 验证 C-F 修复：
// 服务端声明 totalPage 巨大（如 1e9）时，预分配切片必须钳制到 maxTotalPage——
// 否则 make([]pageResult, 1e9+1) 直接 OOM 崩进程。
func TestGetSubmittedCircles_TotalPageClamped(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"code":     1,
				"pageBean": json.RawMessage(submittedPageBean(1, submittedPageSize, 999999999, 1000000000)),
				"dataList": []map[string]any{submittedRecord(1, "首页", 0)},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c, err := client.New(
		client.WithBaseURL(biz.URL),
		client.WithTimeout(5*time.Second),
		client.WithSubmittedPageSize(submittedPageSize),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	// 不能 panic、不能 OOM；应返回首页数据（翻页被钳制）
	records, err := c.GetSubmittedCircles(context.Background(), "tok", "")
	if err != nil {
		t.Fatalf("GetSubmittedCircles: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("首页数据不应丢失")
	}
}
