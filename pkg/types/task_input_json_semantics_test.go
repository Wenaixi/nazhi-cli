package types

import (
	"encoding/json"
	"testing"
)

func TestTaskSubmitInputJSONPreservesExistingFieldsOnPartialPayload(t *testing.T) {
	input := TaskSubmitInput{
		TaskID:  16493,
		Content: "已有内容",
		Hours:   "2",
	}
	if err := json.Unmarshal([]byte(`{"content":"新内容"}`), &input); err != nil {
		t.Fatalf("部分 JSON 解码失败: %v", err)
	}
	if input.TaskID != 16493 || input.Content != "新内容" || input.Hours != "2" {
		t.Fatalf("部分解码不应清空未提供字段: %+v", input)
	}
}

func TestTaskEditInputJSONPreservesExistingFieldsOnPartialPayload(t *testing.T) {
	input := TaskEditInput{
		ID:      5400001,
		TaskID:  16493,
		Content: "已有内容",
		Hours:   "2",
	}
	if err := json.Unmarshal([]byte(`{"content":"新内容"}`), &input); err != nil {
		t.Fatalf("部分 JSON 解码失败: %v", err)
	}
	if input.ID != 5400001 || input.TaskID != 16493 || input.Content != "新内容" || input.Hours != "2" {
		t.Fatalf("部分解码不应清空未提供字段: %+v", input)
	}
}
