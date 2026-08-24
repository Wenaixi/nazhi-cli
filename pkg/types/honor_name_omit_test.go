package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAddHonorPayload_EmptyNameOmitted 锁定：前端 addHonor 表单八键不含 name
// （performanceM.vue:211-220），SDK 空 Name 不应把 name 空串发上线。
// 非空 Name 保留序列化（兼容旧调用方显式传入场景）。
func TestAddHonorPayload_EmptyNameOmitted(t *testing.T) {
	// 空 Name → wire 上不得出现 name 键
	b, err := json.Marshal(&AddHonorPayload{TypeID: 1147, Level: 5, EvaluationAgency: "校"})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if strings.Contains(string(b), `"name"`) {
		t.Fatalf("空 Name 不应序列化出 name 键: %s", b)
	}
	// 非 Name 字段照常输出
	if !strings.Contains(string(b), `"typeId":1147`) {
		t.Fatalf("typeId 丢失: %s", b)
	}

	// 显式非空 Name 仍应输出
	b2, _ := json.Marshal(&AddHonorPayload{Name: "自定义名"})
	if !strings.Contains(string(b2), `"name":"自定义名"`) {
		t.Fatalf("非空 Name 应保留: %s", b2)
	}
}