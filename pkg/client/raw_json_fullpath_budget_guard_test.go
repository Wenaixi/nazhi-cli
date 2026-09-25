// getCirclesJSON 全量路径 make 前预算守卫（CLI-124-01）。
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
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestGetCirclesJSON_FetchGuardBeforeMake 锁定 CLI-124-01：全量路径
// 在 make([]rawResult, declaredPages+1) 前补「页数 × 首页字节」预算守卫，
// 与 limit 路径的 N-04 预估守卫同纪律。
//
// 构造：pageSize=1、首页单条 ~2MB 记录 + 服务端声明 totalNum=64、totalPage=64，
// 64×2MB=128MB 远超 64MB 合并预算。此前该路径真实发出 page2..64 的全量
// errgroup 请求、累积 128MB 后才在 g.Wait 后截断；加守卫后翻页绝不发起，
// 直接返回首页快照。
func TestGetCirclesJSON_FetchGuardBeforeMake(t *testing.T) {
	hugeRecord := []byte(`{"id":999,"payload":"` + strings.Repeat("a", 2<<20) + `"}`)

	var hits int32
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		// 只统计 getStudentCircle 翻页请求（预热/login 等其他请求不计入）
		if r.URL.Path == "/api/studentCircleNew/getStudentCircle" {
			atomic.AddInt32(&hits, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		pn, _ := strconv.Atoi(r.URL.Query().Get("pageNo"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":     1,
			"dataList": []json.RawMessage{hugeRecord},
			"pageBean": map[string]any{
				"pageNo": pn, "pageSize": 1,
				"totalNum": 64, "totalPage": 64,
			},
		})
	})))
	defer biz.Close()

	// pageSize=1 让 totalNum=64 超过单页容量，走全量翻页路径（pageSize=500 时
	// 64<=500 直接返回首页不触发翻页，测试构造就失效了）
	c, perr := client.New(
		client.WithTimeout(5*time.Second),
		client.WithSSOBase(biz.URL),
		client.WithBaseURL(biz.URL),
		client.WithSubmittedPageSize(1),
	)
	if perr != nil {
		t.Fatalf("构造 Client: %v", perr)
	}
	defer c.Close()

	raw, err := c.GetSubmittedCirclesJSON(context.Background(), "token", "")
	if err != nil {
		t.Fatalf("GetSubmittedCirclesJSON: %v", err)
	}
	var arr []json.RawMessage
	if jerr := json.Unmarshal(raw, &arr); jerr != nil {
		t.Fatalf("结果非合法 JSON: %v len=%d", jerr, len(raw))
	}
	// 守卫应返回首页快照（1 条记录）
	if len(arr) != 1 {
		t.Fatalf("守卫应返回首页快照 1 条，实际 %d 条", len(arr))
	}
	// 关键断言：翻页请求绝不发出（仅首页 1 次）——此前会发 64 次
	if got := atomic.LoadInt32(&hits); got > 1 {
		t.Fatalf("守卫后翻页请求 = %d 次（应仅首页 1 次）", got)
	}
}
