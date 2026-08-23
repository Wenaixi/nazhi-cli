package types

import (
	"encoding/json"
	"testing"
)

// TestHoursAcceptsFrontendNumberAndString 验证三条前端响应链兼容数字和数字字符串。
func TestHoursAcceptsFrontendNumberAndString(t *testing.T) {
	cases := []struct {
		name   string
		target any
	}{
		{name: "task", target: new(Task)},
		{name: "circle task metadata", target: new(TaskCircleTypeInfo)},
		{name: "circle record", target: new(CircleRecord)},
	}
	for _, tc := range cases {
		t.Run(tc.name+" number", func(t *testing.T) {
			if err := json.Unmarshal([]byte(`{"hours":0.5}`), tc.target); err != nil {
				t.Fatalf("数字 hours 解码失败: %v", err)
			}
		})
		t.Run(tc.name+" string", func(t *testing.T) {
			if err := json.Unmarshal([]byte(`{"hours":"0.5"}`), tc.target); err != nil {
				t.Fatalf("字符串 hours 解码失败: %v", err)
			}
		})
	}
}
