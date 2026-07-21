package types

import (
	"encoding/json"
	"testing"
)

// TestHonorType_ScoreString 平台 getHonorType 的 score 为展示文案。
// 历史误标 int 会导致 DecodeDataList 整页失败（同类 admissionDate）。
func TestHonorType_ScoreString(t *testing.T) {
	raw := []byte(`{"id":1148,"name":"校三好学生","level_name":"校","level":5,"dimension_name":"学业水平","score":"分数：+5.0"}`)
	var ht HonorType
	if err := json.Unmarshal(raw, &ht); err != nil {
		t.Fatalf("Unmarshal HonorType: %v", err)
	}
	if ht.Score != "分数：+5.0" {
		t.Errorf("Score=%q, want 分数：+5.0", ht.Score)
	}
	if ht.ID != 1148 || ht.Name != "校三好学生" {
		t.Errorf("其它字段: %+v", ht)
	}
}

// TestHonorRecord_ScoreFloat 平台 getHonorByStudentId 的 score 常为 4.0 浮点字面量。
func TestHonorRecord_ScoreFloat(t *testing.T) {
	raw := []byte(`{"id":64205,"type_name":"其他个人荣誉（年级）","level_name":"校","level":5,"status":1,"statusName":"审核通过","get_date":"2026-05-25","score":4.0,"type_id":1291}`)
	var hr HonorRecord
	if err := json.Unmarshal(raw, &hr); err != nil {
		t.Fatalf("Unmarshal HonorRecord: %v", err)
	}
	if hr.Score != 4.0 {
		t.Errorf("Score=%v, want 4.0", hr.Score)
	}
	if hr.ID != 64205 || hr.Status != 1 {
		t.Errorf("其它字段: %+v", hr)
	}
}

// TestHonorRecordList_ScoreFloat 一页中带 4.0 时整列表须成功解码。
func TestHonorRecordList_ScoreFloat(t *testing.T) {
	raw := []byte(`[
		{"id":1,"type_name":"a","level_name":"校","level":5,"score":4.0,"status":1,"statusName":"通过","get_date":"2026-05-25"},
		{"id":2,"type_name":"b","level_name":"校","level":5,"score":5,"status":1,"statusName":"通过","get_date":"2026-05-26"}
	]`)
	var list []HonorRecord
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("list Unmarshal: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].Score != 4.0 || list[1].Score != 5.0 {
		t.Errorf("scores=%v,%v", list[0].Score, list[1].Score)
	}
}
