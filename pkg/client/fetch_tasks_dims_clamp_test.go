package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestFetchTasks_DimsClamped 锁定维度钳制对结构化 FetchTasks 路径的
// 覆盖：FetchTasksJSON 有 maxFetchTasksDims=128 上界（raw_json.go:50），
// 但 FetchTasks（task.go:74-136）全程无钳制——服务端 getDimensions 声明
// 恶意维度数（>128）时，结构化路径会全量并发拉取，与 raw 路径的防护不对称。
//
// 修复前：129 维 → 返回 129 个 task（无截断）→ 本测试 FAIL。
// 修复后：129 维 → 截断到前 128 维，第 129 维不发请求 → 本测试 PASS。
func TestFetchTasks_DimsClamped(t *testing.T) {
	const overDims = 129 // maxFetchTasksDims=128 + 1

	dims := make([]types.Dimension, 0, overDims)
	for i := 1; i <= overDims; i++ {
		dims = append(dims, types.Dimension{ID: int64(i), Name: "dim"})
	}

	// atomic 而非普通 int：httptest 的 handler 由 net/http 为每个请求起
	// goroutine，FetchTasks 又是多路并发拉取，普通 int 的 ++ 是数据竞争
	// （CI 的 -race 步骤会报 WARNING: DATA RACE 并使 job 失败）。
	var statCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			resp := types.UnifiedResponse{Code: 1}
			raw, _ := json.Marshal(dims)
			rawMsg := json.RawMessage(raw)
			resp.DataList = &rawMsg
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/studentCircleNew/getCircleStatistics":
			statCalls.Add(1)
			qid := r.URL.Query().Get("dimensionId")
			if qid == "129" {
				t.Errorf("被截断的第 129 维不应发出 getCircleStatistics 请求")
			}
			resp := types.UnifiedResponse{Code: 1}
			task := types.Task{ID: 1, Name: "task"}
			raw, _ := json.Marshal([]types.Task{task})
			rawMsg := json.RawMessage(raw)
			resp.DataList = &rawMsg
			_ = json.NewEncoder(w).Encode(resp)
		case "/", "/api/studentInfo/getMenu":
			_ = json.NewEncoder(w).Encode(types.UnifiedResponse{Code: 1, Msg: ptr("ok")})
		case "/api/studentInfo/getMyInfo":
			raw := json.RawMessage(`{"id":1,"name":"t","studentNumber":"S1","schoolId":173,"schoolName":"本地测试学校"}`)
			_ = json.NewEncoder(w).Encode(types.UnifiedResponse{Code: 1, ReturnData: &raw})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	c, err := New(WithBaseURL(server.URL), WithSSOBase(server.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tasks, err := c.FetchTasks(t.Context(), "test-token")
	if err != nil {
		t.Fatalf("FetchTasks: %v", err)
	}
	if len(tasks) != maxFetchTasksDims {
		t.Errorf("维度数超过钳制上限时应截断到 %d，实际返回 %d 个 task", maxFetchTasksDims, len(tasks))
	}
	if n := statCalls.Load(); n > maxFetchTasksDims {
		t.Errorf("getCircleStatistics 请求数应 ≤ %d，实际 %d", maxFetchTasksDims, n)
	}
}

// 确保 mock 里用到的辅助 ptr 存在（跨文件同包）。
var _ = ptr
var _ = strings.Contains
