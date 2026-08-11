package main

import (
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func TestDecodeTaskSubmitInputFrontendNumericValues(t *testing.T) {
	input, err := decodeTaskSubmitInput([]byte(`{
		"taskId": 16493,
		"content": "内容",
		"hours": 0.5,
		"level": 5,
		"checkResult": 3,
		"playRole": 3
	}`))
	if err != nil {
		t.Fatalf("CLI 应能解析前端数字字段: %v", err)
	}
	if input.Hours != "0.5" || input.Level != "5" || input.CheckResult != "3" || input.PlayRole != "3" {
		t.Fatalf("数字字段归一错误: hours=%q level=%q checkResult=%q playRole=%q", input.Hours, input.Level, input.CheckResult, input.PlayRole)
	}
}

func TestDecodeTaskEditInputFrontendNumericValues(t *testing.T) {
	input, err := decodeTaskEditInput([]byte(`{
		"id": 5400001,
		"taskId": 16493,
		"content": "修改内容",
		"hours": 1,
		"level": 2,
		"checkResult": 1,
		"playRole": 1
	}`))
	if err != nil {
		t.Fatalf("CLI 应能解析前端编辑数字字段: %v", err)
	}
	if input.ID != 5400001 || input.TaskID != 16493 {
		t.Fatalf("编辑主键字段解析错误: id=%d taskId=%d", input.ID, input.TaskID)
	}
	if input.Hours != "1" || input.Level != "2" || input.CheckResult != "1" || input.PlayRole != "1" {
		t.Fatalf("编辑数字字段归一错误: hours=%q level=%q checkResult=%q playRole=%q", input.Hours, input.Level, input.CheckResult, input.PlayRole)
	}
}

func TestDecodeTaskInputJSONRejectsNonScalarNumericFields(t *testing.T) {
	var input types.TaskSubmitInput
	if err := decodeTaskInputJSON([]byte(`{"taskId":1,"content":"内容","hours":true}`), &input); err == nil {
		t.Fatal("布尔 hours 应被拒绝")
	}
}

func TestDecodeTaskSubmitInputFrontendAliases(t *testing.T) {
	input, err := decodeTaskSubmitInput([]byte(`{
		"circleTaskId": 16493,
		"content": "内容",
		"pictureList": [12345],
		"hours": 0.5,
		"level": 5,
		"checkResult": 3,
		"playRole": 3
	}`))
	if err != nil {
		t.Fatalf("CLI 应能解析前端写实字段别名: %v", err)
	}
	if input.TaskID != 16493 || len(input.ImageIDs) != 1 || input.ImageIDs[0] != 12345 {
		t.Fatalf("前端字段别名归一错误: taskId=%d imageIDs=%v", input.TaskID, input.ImageIDs)
	}
}

func TestDecodeTaskEditInputFrontendAliasesDoNotOverrideCanonicalFields(t *testing.T) {
	input, err := decodeTaskEditInput([]byte(`{
		"id": 5400001,
		"taskId": 16493,
		"circleTaskId": 99999,
		"content": "修改内容",
		"imageIDs": [67890],
		"pictureList": [12345]
	}`))
	if err != nil {
		t.Fatalf("CLI 应能解析前端编辑字段别名: %v", err)
	}
	if input.TaskID != 16493 || len(input.ImageIDs) != 1 || input.ImageIDs[0] != 67890 {
		t.Fatalf("规范字段不应被别名覆盖: taskId=%d imageIDs=%v", input.TaskID, input.ImageIDs)
	}
}

func TestDecodeTaskSubmitInputCaseInsensitiveNumericFields(t *testing.T) {
	input, err := decodeTaskSubmitInput([]byte(`{
		"taskId": 16493,
		"content": "内容",
		"Hours": 0.5,
		"LEVEL": 5,
		"CheckResult": 3,
		"PlayRole": 3
	}`))
	if err != nil {
		t.Fatalf("大小写变体应保持 JSON 字段兼容: %v", err)
	}
	if input.Hours != "0.5" || input.Level != "5" || input.CheckResult != "3" || input.PlayRole != "3" {
		t.Fatalf("大小写变体归一错误: hours=%q level=%q checkResult=%q playRole=%q", input.Hours, input.Level, input.CheckResult, input.PlayRole)
	}
}
