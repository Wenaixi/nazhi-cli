package types

import (
	"encoding/json"
	"testing"
)

func TestAddHonorPayloadUnmarshalJSONAcceptsNumericCertificateID(t *testing.T) {
	var payload AddHonorPayload
	if err := json.Unmarshal([]byte(`{"typeId":1147,"level":5,"certImgAttachmentId":4383235}`), &payload); err != nil {
		t.Fatalf("前端 number certImgAttachmentId 应可解析: %v", err)
	}
	if payload.CertImgAttachmentID != "4383235" {
		t.Fatalf("证书附件 ID 归一错误: %q", payload.CertImgAttachmentID)
	}
}

func TestAddHonorPayloadUnmarshalJSONAcceptsStringCertificateID(t *testing.T) {
	var payload AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":"4383235"}`), &payload); err != nil {
		t.Fatalf("字符串 certImgAttachmentId 应可解析: %v", err)
	}
	if payload.CertImgAttachmentID != "4383235" {
		t.Fatalf("证书附件 ID 不应改变: %q", payload.CertImgAttachmentID)
	}
}

func TestAddHonorPayloadUnmarshalJSONRejectsNonIntegerCertificateID(t *testing.T) {
	var payload AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":4383235.5}`), &payload); err == nil {
		t.Fatal("非整数证书附件 ID 应返回错误")
	}
}
