package types

import (
	"encoding/json"
	"testing"
)

// TestTypicalCaseRecord_StringTypeRoleLevel 验证前端返回 string 类型的 type/role/level 能成功解码。
// 审计背景：前端列表响应可能返回 "1" 字符串，裸 int 会导致 DecodeDataList 整页失败。
func TestTypicalCaseRecord_StringTypeRoleLevel(t *testing.T) {
	raw := `{"id":1,"title":"t","type":"1","typeName":"研究性学习报告","role":"2","roleName":"参与者","level":"5","levelName":"学校","attachmentId":0,"status":0,"statusName":"未审核"}`
	var rec TypicalCaseRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("string type/role/level 解码失败（期望成功）: %v", err)
	}
	if rec.Type != 1 {
		t.Fatalf("Type 期望 1, 得到 %d", rec.Type)
	}
	if rec.Role != 2 {
		t.Fatalf("Role 期望 2, 得到 %d", rec.Role)
	}
	if rec.Level != 5 {
		t.Fatalf("Level 期望 5, 得到 %d", rec.Level)
	}
}

// TestTypicalCaseRecord_NumberTypeRoleLevel 对照组：number 类型仍需成功（不回归）。
func TestTypicalCaseRecord_NumberTypeRoleLevel(t *testing.T) {
	raw := `{"id":2,"title":"t2","type":1,"role":2,"level":5,"attachmentId":123,"status":0}`
	var rec TypicalCaseRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("number type/role/level 解码失败: %v", err)
	}
	if rec.Type != 1 || rec.Role != 2 || rec.Level != 5 {
		t.Fatalf("期望 Type=1 Role=2 Level=5, 得到 %d %d %d", rec.Type, rec.Role, rec.Level)
	}
	if rec.AttachmentID != 123 {
		t.Fatalf("AttachmentID 期望 123, 得到 %d", rec.AttachmentID)
	}
}

// TestTypicalCaseRecord_DecodeDataList_StringTypes 验证 DecodeDataList 在 string 数字时整页不失败。
func TestTypicalCaseRecord_DecodeDataList_StringTypes(t *testing.T) {
	payload := `{"code":1,"msg":"ok","dataList":[{"id":1,"title":"a","type":"1","role":"1","level":"2","status":0},{"id":2,"title":"b","type":"2","role":"2","level":"5","status":0}],"pageBean":{"pageNo":1,"pageSize":10,"totalNum":2,"totalPage":1}}`
	resp, err := DecodeResponse([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeResponse 失败: %v", err)
	}
	records, err := DecodeDataList[TypicalCaseRecord](resp)
	if err != nil {
		t.Fatalf("DecodeDataList 期望成功（string 兼容），但失败: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("期望 2 条记录, 得到 %d", len(records))
	}
	if records[0].Type != 1 || records[0].Level != 2 {
		t.Fatalf("第一条期望 Type=1 Level=2, 得到 %d %d", records[0].Type, records[0].Level)
	}
	if records[1].Type != 2 || records[1].Level != 5 {
		t.Fatalf("第二条期望 Type=2 Level=5, 得到 %d %d", records[1].Type, records[1].Level)
	}
}

// TestTypicalCaseRecord_MixedAndNull 覆盖混合类型、null、空字符串、float 1.0 等边界。
func TestTypicalCaseRecord_MixedAndNull(t *testing.T) {
	cases := []struct {
		name  string
		json  string
		check func(TypicalCaseRecord) bool
	}{
		{
			name:  "string with spaces",
			json:  `{"id":1,"type":" 1 ","role":" 2 ","level":" 3 "}`,
			check: func(r TypicalCaseRecord) bool { return r.Type == 1 && r.Role == 2 && r.Level == 3 },
		},
		{
			name:  "null",
			json:  `{"id":1,"type":null,"role":null,"level":null}`,
			check: func(r TypicalCaseRecord) bool { return r.Type == 0 && r.Role == 0 && r.Level == 0 },
		},
		{
			name:  "empty string",
			json:  `{"id":1,"type":"","role":"","level":""}`,
			check: func(r TypicalCaseRecord) bool { return r.Type == 0 && r.Role == 0 && r.Level == 0 },
		},
		{
			name:  "float number 1.0",
			json:  `{"id":1,"type":1.0,"role":2.0,"level":5.0}`,
			check: func(r TypicalCaseRecord) bool { return r.Type == 1 && r.Role == 2 && r.Level == 5 },
		},
		{
			name:  "mixed string and number",
			json:  `{"id":1,"type":"1","role":2,"level":"5"}`,
			check: func(r TypicalCaseRecord) bool { return r.Type == 1 && r.Role == 2 && r.Level == 5 },
		},
		{
			name:  "attachmentId string",
			json:  `{"id":1,"type":1,"role":1,"level":1,"attachmentId":"123"}`,
			check: func(r TypicalCaseRecord) bool { return r.AttachmentID == 123 },
		},
		{
			name:  "attachmentId empty string",
			json:  `{"id":1,"type":1,"role":1,"level":1,"attachmentId":""}`,
			check: func(r TypicalCaseRecord) bool { return r.AttachmentID == 0 },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rec TypicalCaseRecord
			if err := json.Unmarshal([]byte(c.json), &rec); err != nil {
				t.Fatalf("解码失败: %v json=%s", err, c.json)
			}
			if !c.check(rec) {
				t.Fatalf("校验失败 json=%s 得到 %+v", c.json, rec)
			}
		})
	}
}
