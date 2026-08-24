package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestAddHonor_TypeNameLookupMiss_StillSubmits 反查未命中时不得中止提交：
// 历史 typeId 可能已从字典下架/改名（getHonorTypeForSelect 不含该项），
// 前端行为是保留原 typeName 照常提交交服务端裁决；SDK 硬失败比前端严格，
// 会在「payload 不含 typeName + 下拉请求抖动」时阻断本可成功的申报。
func TestAddHonor_TypeNameLookupMiss_StillSubmits(t *testing.T) {
	var gotBody map[string]any
	biz := httptest.NewServer(http.HandlerFunc(warmupBizHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/studentMoralEduNew/getHonorTypeForSelect":
			w.Header().Set("Content-Type", "application/json")
			// 选项表中没有 payload 的 typeId=9999
			resp := map[string]any{
				"code":       1,
				"dataList":   []map[string]any{{"label": "校三好学生", "value": 1147}},
				"returnData": []map[string]any{{"label": "校", "value": 5}},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		case "/api/studentMoralEduNew/addHonor":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"添加成功"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})))
	defer biz.Close()

	c := newTestClient(nil, biz, nil)
	err := c.AddHonor(context.Background(), "test-token", types.AddHonorPayload{
		TypeID:           9999,
		Level:            5,
		EvaluationAgency: "示例中学",
		GetDate:          "2026-06-30",
	})
	if err != nil {
		t.Fatalf("反查未命中应放行提交, 实际报错: %v", err)
	}
	if gotBody == nil {
		t.Fatal("请求未被发出")
	}
	if v, ok := gotBody["typeName"]; !ok || v != "" {
		t.Errorf("未命中时 typeName 应保持空串原样发出, 实际 %v (present=%v)", v, ok)
	}
}
