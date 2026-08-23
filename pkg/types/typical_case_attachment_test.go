package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAddTypicalCasePayload_EmptyAttachmentIDFromFrontend(t *testing.T) {
	var payload AddTypicalCasePayload
	if err := json.Unmarshal([]byte(`{"title":"案例","attachmentId":""}`), &payload); err != nil {
		t.Fatalf("前端空附件 ID 应可解析: %v", err)
	}
	if payload.AttachmentID != 0 {
		t.Fatalf("空附件 ID 应归一为 0，实际 %d", payload.AttachmentID)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("重新编码失败: %v", err)
	}
	if strings.Contains(string(encoded), `"attachmentId"`) {
		t.Fatalf("无附件时不应发送 attachmentId，实际 %s", encoded)
	}
}

func TestAddTypicalCasePayload_PreservesAttachmentID(t *testing.T) {
	var payload AddTypicalCasePayload
	if err := json.Unmarshal([]byte(`{"attachmentId":5139876}`), &payload); err != nil {
		t.Fatalf("数字附件 ID 应可解析: %v", err)
	}
	if payload.AttachmentID != 5139876 {
		t.Fatalf("附件 ID 解析错误: %d", payload.AttachmentID)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("重新编码失败: %v", err)
	}
	if !strings.Contains(string(encoded), `"attachmentId":5139876`) {
		t.Fatalf("有附件时应发送数字 ID，实际 %s", encoded)
	}
}

func TestAddTypicalCasePayloadUnmarshalPreservesAbsentFields(t *testing.T) {
	payload := AddTypicalCasePayload{
		Type:           "2",
		TypeName:       "社会调查报告",
		AttachmentID:   5139876,
		AttachmentName: "附件.pdf",
	}
	if err := json.Unmarshal([]byte(`{"title":"更新标题"}`), &payload); err != nil {
		t.Fatalf("部分 JSON 应可解析: %v", err)
	}
	if payload.Title != "更新标题" {
		t.Fatalf("标题未更新: %q", payload.Title)
	}
	if payload.Type != "2" || payload.TypeName != "社会调查报告" {
		t.Fatalf("缺失字段不应被清零，类型=%q，类型名=%q", payload.Type, payload.TypeName)
	}
	if payload.AttachmentID != 5139876 || payload.AttachmentName != "附件.pdf" {
		t.Fatalf("缺失附件字段不应被清零，ID=%d，名称=%q", payload.AttachmentID, payload.AttachmentName)
	}
}
