package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestFetchTasks_ViaParallelDims_Regression 锁定迁移到 ParallelDims 前后的行为一致性。
// 覆盖：正常聚合、单维度失败 partial、id=0 跳过。
func TestFetchTasks_ViaParallelDims_Regression(t *testing.T) {
	tests := []struct {
		name                 string
		dims                 []types.Dimension
		failDimID            int64
		wantTasksAtLeast     int
		wantBusinessRejected bool
	}{
		{"2维度全成功", []types.Dimension{{ID: 1, Name: "d1"}, {ID: 2, Name: "d2"}}, 0, 2, false},
		{"单维度失败partial", []types.Dimension{{ID: 1, Name: "d1"}, {ID: 2, Name: "d2"}}, 2, 1, true},
		{"含id0跳过", []types.Dimension{{ID: 0, Name: "全部"}, {ID: 1, Name: "d1"}, {ID: 2, Name: "d2"}}, 0, 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := mockFetchTasksServer(t, tc.dims, tc.failDimID)
			defer srv.Close()
			c, _ := New(WithBaseURL(srv.URL), WithSSOBase(srv.URL), WithTimeout(5*1e9))
			defer c.Close()
			c.sm.StoreToken("test-token")
			// 注入维度：通过 fetchDimensions 的 mock 维度返回
			// 实际用 httptest 模拟 getDimensions + getCircleStatistics 两段
			tasks, err := c.FetchTasks(context.Background(), "test-token")
			if tc.wantBusinessRejected {
				if err == nil || !errors.Is(err, ErrBusinessRejected) {
					t.Fatalf("期望 ErrBusinessRejected，got err=%v", err)
				}
				if len(tasks) < tc.wantTasksAtLeast {
					t.Fatalf("partial 应保留已成功任务，want>=%d got %d", tc.wantTasksAtLeast, len(tasks))
				}
			} else {
				if err != nil {
					t.Fatalf("期望成功，got err=%v", err)
				}
				if len(tasks) < tc.wantTasksAtLeast {
					t.Fatalf("want>=%d tasks, got %d", tc.wantTasksAtLeast, len(tasks))
				}
			}
		})
	}
}

func mockFetchTasksServer(t *testing.T, dims []types.Dimension, failDimID int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/studentCircleNew/getDimensions":
			resp := types.UnifiedResponse{Code: 1}
			raw, _ := json.Marshal(dims)
			rawMsg := json.RawMessage(raw)
			resp.DataList = &rawMsg
			_ = json.NewEncoder(w).Encode(resp)
		case "/api/studentCircleNew/getCircleStatistics":
			qid := r.URL.Query().Get("dimensionId")
			var dimID int64
			for _, d := range dims {
				if qid == sprintInt64(d.ID) {
					dimID = d.ID
					break
				}
			}
			if dimID == failDimID {
				resp := types.UnifiedResponse{Code: 0, Msg: ptr("dim fail")}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			resp := types.UnifiedResponse{Code: 1}
			task := types.Task{ID: dimID*10 + 1, Name: "task"}
			raw, _ := json.Marshal([]types.Task{task})
			rawMsg := json.RawMessage(raw)
			resp.DataList = &rawMsg
			_ = json.NewEncoder(w).Encode(resp)
		case "/", "/api/studentInfo/getMenu":
			_ = json.NewEncoder(w).Encode(types.UnifiedResponse{Code: 1, Msg: ptr("ok")})
		case "/api/studentInfo/getMyInfo":
			raw := json.RawMessage(`{"id":1,"name":"t","studentNumber":"S1"}`)
			_ = json.NewEncoder(w).Encode(types.UnifiedResponse{Code: 1, ReturnData: &raw})
		default:
			t.Logf("mockFetchTasksServer 未命中 %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

func sprintInt64(v int64) string { return strconv.FormatInt(v, 10) }
