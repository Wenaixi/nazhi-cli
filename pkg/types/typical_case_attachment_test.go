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
