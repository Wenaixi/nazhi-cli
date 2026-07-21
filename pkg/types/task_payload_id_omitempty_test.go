package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTaskAddCirclePayload_IDOmitEmpty 新增提交时 ID=nil 不应序列化出 "id":null。
func TestTaskAddCirclePayload_IDOmitEmpty(t *testing.T) {
	payload := TaskAddCirclePayload{
		// ID 零值 nil
		Name:         "n",
		Content:      "c",
		CircleTaskID: 1,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `"id"`) {
		t.Fatalf("ID=nil 时 JSON 不应含 id 字段，实际: %s", s)
	}
}

// TestTaskAddCirclePayload_IDPresentWhenSet 编辑时 ID 非 nil 应出现在 JSON。
func TestTaskAddCirclePayload_IDPresentWhenSet(t *testing.T) {
	id := int64(5464109)
	payload := TaskAddCirclePayload{ID: &id, Content: "c"}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal 失败: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"id":5464109`) {
		t.Fatalf("ID 非 nil 时应序列化为 id:5464109，实际: %s", s)
	}
}
