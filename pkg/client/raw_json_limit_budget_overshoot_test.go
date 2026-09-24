// getCirclesLimitJSON 合并后累积字节复核截断（CLI-120-1）。
package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestGetCirclesLimitJSON_PostConcatBudgetOvershoot 锁定 CLI-120-1：翻页完成后
// 若累积原始字节越过 maxAssembleBuffer 预算（估算假设「每页 ≤ 首页字节」
// 被服务端分页异常打破——首页极小、后续页灌满大记录），必须截断到已合并合法
// 前缀，不得把超预算累积全量交给 assembleCirclesLimitJSON 无限增长。
//
// 构造：totalPage=24、pageSize=1；首页 1 条小记录作估算基，page2..24 每页
// 单条 ~3MB 记录（首页字节 ×24 的估算远小于 24×3MB 的真实累积）。
// 预估 24×30B 不触发预翻页钳制 → 真翻页 → capAssembledSlice 实测累积
// 69MB > 64MB 预算 → 截断到合法前缀（budgetPage 降到 ≤21 页）。
func TestGetCirclesLimitJSON_PostConcatBudgetOvershoot(t *testing.T) {
	// 3MB 单条记录（< maxResponseBodySize 4MB 请求守卫），复用避免重复分配
	bigRecord := `{"id":999,"payload":"` + strings.Repeat("a", 3<<20) + `"}`

	var pageHits int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/studentCircleNew/getStudentCircle" {
			http.NotFound(w, r)
			return
		}
		pn, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		atomic.AddInt32(&pageHits, 1)
		w.Header().Set("Content-Type", "application/json")
		records := []any{map[string]any{"id": pn}}
		if pn > 1 {
			// page2.. 返回大记录——服务端分页异常（后续页远大于首页）
			records = []any{json.RawMessage(bigRecord)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":     1,
			"dataList": records,
			"pageBean": map[string]any{
				"pageNo": pn, "pageSize": pageSize,
				"totalNum": 24, "totalPage": 24,
			},
		})
	})))
	defer biz.Close()

	c, perr := client.New(client.WithBaseURL(biz.URL), client.WithSSOBase(biz.URL), client.WithSubmittedPageSize(1))
	if perr != nil {
		t.Fatalf("构造 Client: %v", perr)
	}
	defer c.Close()

	// limit=24 → endPage=24。预翻页估算（24×首页字节）不触发钳制，
	// 必须由合并后的 capAssembledSlice 复核截断。
	raw, pb, err := c.GetSubmittedCirclesLimitJSON(context.Background(), "token", 0, 24, "")
	if err != nil {
		t.Fatalf("GetSubmittedCirclesLimitJSON: %v", err)
	}
	if pb == nil || pb.TotalNum != 24 {
		t.Fatalf("期望 TotalNum=24, pb=%+v", pb)
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Fatalf("结果非合法 JSON: %v len=%d", jerr, len(raw))
	}
	// 预算复核后应截断到合法前缀（<24 条），证明 capAssembledSlice 真的参与
	if len(arr) == 0 || len(arr) >= 24 {
		t.Fatalf("预算截断未生效: len(arr)=%d（应 >0 且 <24）", len(arr))
	}
	// 第一页（首页小记录）必须保留在结果内
	if arr[0]["id"] != float64(1) {
		t.Fatalf("首页记录丢失: arr[0]=%+v", arr[0])
	}
	// 不超出声明页数翻页
	if got := atomic.LoadInt32(&pageHits); got > 24 {
		t.Fatalf("请求页数 = %d, 期望 ≤24", got)
	}
}
