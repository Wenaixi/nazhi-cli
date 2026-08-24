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
// （与提交 addSelfEvaluation 的 studentComment 请求键一致）。
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
	// snake 主读（平台 dataMap 真实形态），camel 兼容——与 client.normalizeSelfEvalStatus 口径一致
	studentComment, present, err := firstJSONString(raw, "student_comment", "studentComment")
	if err != nil {
		return fmt.Errorf("SelfEvalStatus.studentComment: %w", err)
	}
	if present {
		s.StudentComment = studentComment
	}
	teacherComment, present, err := firstJSONString(raw, "teacher_comment", "teacherComment")
	if err != nil {
		return fmt.Errorf("SelfEvalStatus.teacherComment: %w", err)
	}
	if present {
		s.TeacherComment = teacherComment
	}
	return nil
}

func firstJSONString(raw map[string]json.RawMessage, keys ...string) (string, bool, error) {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok {
			continue
		}
		trimmed := bytes.TrimSpace(v)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			continue
		}
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return "", true, err
		}
		return s, true, nil
	}
	return "", false, nil
}
