package types

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SelfEvalStatus 是自我评价状态。
//
// 查询接口 dataMap 前端读 student_comment / teacher_comment（mainLeft.vue、selfgaintloss.vue）；
// 部分 mock / returnData 为 camelCase。Unmarshal 双键兼容；Marshal 输出 camelCase
//（与提交 addSelfEvaluation 的 studentComment 请求键一致）。
type SelfEvalStatus struct {
	ID             int64  `json:"id"`
	StudentComment string `json:"studentComment"`
	TeacherComment string `json:"teacherComment"`
}

// UnmarshalJSON 兼容 student_comment / studentComment 与 teacher_comment / teacherComment。
func (s *SelfEvalStatus) UnmarshalJSON(data []byte) error {
	if s == nil {
		return fmt.Errorf("SelfEvalStatus: UnmarshalJSON on nil pointer")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["id"]; ok {
		if err := json.Unmarshal(v, &s.ID); err != nil {
			return fmt.Errorf("SelfEvalStatus.id: %w", err)
		}
	}
	s.StudentComment = firstJSONString(raw, "studentComment", "student_comment")
	s.TeacherComment = firstJSONString(raw, "teacherComment", "teacher_comment")
	return nil
}

func firstJSONString(raw map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok || len(v) == 0 || bytes.Equal(v, []byte("null")) {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
	}
	return ""
}
