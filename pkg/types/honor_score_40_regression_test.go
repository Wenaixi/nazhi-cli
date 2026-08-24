package types

import (
	"encoding/json"
	"testing"
)

// TestHonorRecord_Score40_Regression 回归测试：HonorRecord.Score 必须为 float64。
//
// 平台 getHonorByStudentId 列表常见 score:4.0 浮点字面量，encoding/json 会拒绝 4.0→int。
// 若误标 int 会导致 DecodeDataList 整页失败（与 HonorType.score string / admissionDate 同类陷阱）。
// 本用例固定 4.0 字面量，确保解码不失败（score 类型已正确为 float64 的回归保障）。
func TestHonorRecord_Score40_Regression(t *testing.T) {
	raw := []byte(`{"id":1,"type_name":"校三好学生","level_name":"校","level":5,"dimension_name":"思想品德","status":1,"statusName":"通过","get_date":"2026-06-30","evaluation_agency":"示例中学","score":4.0}`)
	var rec HonorRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("HonorRecord score=4.0 解码失败（回归）: %v", err)
	}
	if rec.Score != 4.0 {
		t.Fatalf("Score 期望 4.0，实际 %v", rec.Score)
	}
	// 额外验证：score 为整数 5 时也能解码（平台可能返回 5 / 5.0 混用）
	var rec2 HonorRecord
	if err := json.Unmarshal([]byte(`{"id":2,"type_name":"b","score":5}`), &rec2); err != nil {
		t.Fatalf("HonorRecord score=5 解码失败: %v", err)
	}
	if rec2.Score != 5.0 {
		t.Fatalf("Score 期望 5.0，实际 %v", rec2.Score)
	}
}

// TestAddHonorPayload_Score40_Inbound AddHonorPayload.Score 此前为裸 int：
// 用户从列表记录复制 score:4.0 构造 honor add --payload 时 json.Unmarshal
// 报「cannot unmarshal number 4.0 ... of type int」参数错误。改 float64 后
// 出站序列化 0 与前端 form 默认逐字节一致，入站兼容 4.0 字面量。
func TestAddHonorPayload_Score40_Inbound(t *testing.T) {
	raw := []byte(`{"typeId":1147,"typeName":"校三好学生","level":5,"evaluationAgency":"示例中学","getDate":"2026-06-30","score":4.0}`)
	var p AddHonorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("AddHonorPayload score=4.0 入站解码失败: %v", err)
	}
	if p.Score != 4.0 {
		t.Fatalf("Score 期望 4.0，实际 %v", p.Score)
	}

	// 整数与零值路径不回归
	var p2 AddHonorPayload
	if err := json.Unmarshal([]byte(`{"typeId":1,"score":5}`), &p2); err != nil {
		t.Fatalf("score=5 解码失败: %v", err)
	}
	if p2.Score != 5.0 {
		t.Fatalf("Score 期望 5.0，实际 %v", p2.Score)
	}

	// 出站：零值序列化仍为 "score":0（与前端 form 默认一致）
	out, err := json.Marshal(AddHonorPayload{TypeID: 1})
	if err != nil {
		t.Fatalf("出站序列化失败: %v", err)
	}
	if !json.Valid(out) {
		t.Fatal("出站 JSON 非法")
	}
}

// TestHonorRecordList_Score40_Regression 列表整页含 4.0 时 DecodeDataList 不得整页失败。
func TestHonorRecordList_Score40_Regression(t *testing.T) {
	raw := []byte(`[{"id":1,"type_name":"a","score":4.0},{"id":2,"type_name":"b","score":4.0},{"id":3,"type_name":"c","score":5}]`)
	var list []HonorRecord
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("HonorRecord 列表 score 含 4.0 解码失败: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("列表长度期望 3，实际 %d", len(list))
	}
	if list[0].Score != 4.0 || list[1].Score != 4.0 || list[2].Score != 5.0 {
		t.Fatalf("Score 值不符合预期: %v %v %v", list[0].Score, list[1].Score, list[2].Score)
	}
	// 通过 UnifiedResponse.DecodeDataList 路径同样验证
	dataList := json.RawMessage(raw)
	resp := UnifiedResponse{Code: 1, DataList: &dataList}
	records, err := DecodeDataList[HonorRecord](resp)
	if err != nil {
		t.Fatalf("DecodeDataList HonorRecord 失败: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("DecodeDataList 长度期望 3，实际 %d", len(records))
	}
}
