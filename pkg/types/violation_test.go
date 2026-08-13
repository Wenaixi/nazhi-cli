package types

import (
	"encoding/json"
	"testing"
)

// TestViolationRecordMatchesFrontendJSON 验证违规记录按前端真实字段和类型解码。
func TestViolationRecordMatchesFrontendJSON(t *testing.T) {
	input := []byte(`{"id":9001,"studentName":"测试学生","className":"一班","gradeName":"高一","typeName":"迟到","name":"上课迟到","fromTableName":"校级","score":1.5,"attachmentId":null,"getDateStr":"2026-08-01","creatorName":"班主任","ifshow":"是"}`)

	var record ViolationRecord
	if err := json.Unmarshal(input, &record); err != nil {
		t.Fatalf("解码违规记录失败: %v", err)
	}
	if record.ID != 9001 || record.Score != 1.5 || record.AttachmentID != nil {
		t.Fatalf("违规记录字段解析错误: %+v", record)
	}
	if record.StudentName != "测试学生" || record.FromTableName != "校级" || record.IfShow != "是" {
		t.Fatalf("违规记录文本字段解析错误: %+v", record)
	}
}

// TestViolationTypeMatchesFrontendJSON 验证违规类型下拉项字段。
func TestViolationTypeMatchesFrontendJSON(t *testing.T) {
	var item ViolationType
	if err := json.Unmarshal([]byte(`{"id":7,"name":"迟到"}`), &item); err != nil {
		t.Fatalf("解码违规类型失败: %v", err)
	}
	if item.ID != 7 || item.Name != "迟到" {
		t.Fatalf("违规类型字段解析错误: %+v", item)
	}
}

// TestFlexFloatAcceptsFrontendNumberForms 验证平台数字和数字字符串都能解码。
func TestFlexFloatAcceptsFrontendNumberForms(t *testing.T) {
	for _, input := range []string{`1.5`, `"1.5"`, `null`} {
		var value FlexFloat
		if err := json.Unmarshal([]byte(input), &value); err != nil {
			t.Fatalf("FlexFloat 解码 %s 失败: %v", input, err)
		}
		if input != `null` && value.Float64() != 1.5 {
			t.Fatalf("FlexFloat 解码值错误: input=%s value=%v", input, value.Float64())
		}
	}

	encoded, err := json.Marshal(FlexFloat(1.5))
	if err != nil {
		t.Fatalf("FlexFloat 序列化失败: %v", err)
	}
	if string(encoded) != "1.5" {
		t.Fatalf("FlexFloat 应序列化为 JSON 数字，实际 %s", encoded)
	}
}
