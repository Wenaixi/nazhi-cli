// assembleCirclesJSON 白盒测试：第一页空数组时不得产生 leading comma 非法 JSON。
package client

import (
	"encoding/json"
	"testing"
)

// TestAssembleCirclesJSON_EmptyFirstPage_NoLeadingComma 锁定：
// page1=[]、page2 有数据时，拼接结果必须是合法 JSON（如 [{...}]），不能是 [,{...}]。
func TestAssembleCirclesJSON_EmptyFirstPage_NoLeadingComma(t *testing.T) {
	raw1 := []byte("[]")
	results := make([]rawResult, 3)
	results[2] = rawResult{raw: []byte(`[{"id":200}]`)}

	out, err := assembleCirclesJSON(raw1, results, 2, nil)
	if err != nil {
		t.Fatalf("assembleCirclesJSON 不应返回 error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("结果不应为空")
	}
	// 非法形态：[,{...}]
	if string(out)[0:2] == "[," {
		t.Fatalf("leading comma 非法 JSON: %s", out)
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(out, &arr); jerr != nil {
		t.Fatalf("拼接结果必须是合法 JSON 数组: body=%s err=%v", out, jerr)
	}
	if len(arr) != 1 {
		t.Fatalf("期望 1 条记录, 得到 %d body=%s", len(arr), out)
	}
	if id, _ := arr[0]["id"].(float64); id != 200 {
		t.Errorf("期望 id=200, 得到 %v", arr[0]["id"])
	}
}

// TestAssembleCirclesJSON_EmptyFirstPage_PartialPath 部分失败路径同样不得 leading comma。
func TestAssembleCirclesJSON_EmptyFirstPage_PartialPath(t *testing.T) {
	raw1 := []byte("[]")
	results := make([]rawResult, 4)
	results[2] = rawResult{raw: []byte(`[{"id":1}]`)}
	// page3 缺失（模拟失败页）

	out, err := assembleCirclesJSON(raw1, results, 3, errPartialStub{})
	if err == nil {
		t.Fatal("partialErr 非 nil 时应透传 error")
	}
	var arr []map[string]any
	if jerr := json.Unmarshal(out, &arr); jerr != nil {
		t.Fatalf("partial 路径结果仍须合法 JSON: body=%s err=%v", out, jerr)
	}
	if len(arr) != 1 {
		t.Fatalf("期望 1 条已合并记录, 得到 %d body=%s", len(arr), out)
	}
}

// errPartialStub 仅作 partialErr 占位。
type errPartialStub struct{}

func (errPartialStub) Error() string { return "partial" }
