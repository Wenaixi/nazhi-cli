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
	if payload.CertImgAttachmentID != 4383235 {
		t.Fatalf("证书附件 ID 归一错误: %d", payload.CertImgAttachmentID)
	}
}

func TestAddHonorPayloadUnmarshalJSONAcceptsStringCertificateID(t *testing.T) {
	var payload AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":"4383235"}`), &payload); err != nil {
		t.Fatalf("字符串 certImgAttachmentId 应可解析: %v", err)
	}
	if payload.CertImgAttachmentID != 4383235 {
		t.Fatalf("证书附件 ID 不应改变: %d", payload.CertImgAttachmentID)
	}
}

func TestAddHonorPayloadUnmarshalJSONPreservesMissingFields(t *testing.T) {
	payload := AddHonorPayload{
		Name:                "校三好学生",
		TypeID:              1147,
		TypeName:            "校三好学生",
		Level:               5,
		EvaluationAgency:    "福州市教育局",
		GetDate:             "2026-05-25",
		CertImgAttachmentID: 999,
		Score:               5,
	}
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":4383235}`), &payload); err != nil {
		t.Fatalf("number certImgAttachmentId 应可解析: %v", err)
	}
	if payload.CertImgAttachmentID != 4383235 {
		t.Fatalf("证书附件 ID 归一错误: %d", payload.CertImgAttachmentID)
	}
	if payload.Name != "校三好学生" || payload.TypeID != 1147 || payload.TypeName != "校三好学生" || payload.Level != 5 ||
		payload.EvaluationAgency != "福州市教育局" || payload.GetDate != "2026-05-25" || payload.Score != 5 {
		t.Fatalf("缺失字段不应被覆盖: %+v", payload)
	}
}

func TestAddHonorPayloadUnmarshalJSONDistinguishesNullAndMissingCertificateID(t *testing.T) {
	payload := AddHonorPayload{CertImgAttachmentID: 4383235}
	if err := json.Unmarshal([]byte(`{}`), &payload); err != nil {
		t.Fatalf("缺失证书附件 ID 应可解析: %v", err)
	}
	if payload.CertImgAttachmentID != 4383235 {
		t.Fatalf("缺失证书附件 ID 不应覆盖原值: %d", payload.CertImgAttachmentID)
	}
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":null}`), &payload); err != nil {
		t.Fatalf("null 证书附件 ID 应可解析: %v", err)
	}
	if payload.CertImgAttachmentID != 0 {
		t.Fatalf("null 证书附件 ID 应清空原值: %d", payload.CertImgAttachmentID)
	}
}

func TestAddHonorPayloadUnmarshalJSONRejectsNonIntegerCertificateID(t *testing.T) {
	var payload AddHonorPayload
	if err := json.Unmarshal([]byte(`{"certImgAttachmentId":4383235.5}`), &payload); err == nil {
		t.Fatal("非整数证书附件 ID 应返回错误")
	}
}
