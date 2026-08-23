package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// 典型案例角色常量（对应服务端 role 字段）。
const (
	TypicalCaseRoleHost        = "1" // 负责人
	TypicalCaseRoleParticipant = "2" // 参与者
)

// AddTypicalCasePayload 是 addTypicalCase 接口的请求体。
//
// type/role/level 在请求体中是 JSON 字符串（HAR 确认），
// 与列表响应中的整数类型不同。
//
// 用户输入（对齐 classiccanter.vue v-model）：
// Title/Type/TeacherName/PartnerName/Role/Remark/Content/Level + 附件。
// SDK 自动（AddTypicalCase 内）：TypeName/RoleName/LevelName 由代码映射；
// 已显式填写的 *Name 不会被覆盖。调用方不必手填展示名。
type AddTypicalCasePayload struct {
	Title          string `json:"title"`                  // 标题（用户）
	Type           string `json:"type"`                   // 材料类别代码（用户选，"1"…）
	TypeName       string `json:"typeName"`               // 材料类别名称（SDK 可自动填）
	TeacherName    string `json:"teacherName"`            // 指导教师（用户）
	PartnerName    string `json:"partnerName"`            // 合作者（用户）
	Role           string `json:"role"`                   // 角色代码（用户选）
	RoleName       string `json:"roleName"`               // 角色名称（SDK 可自动填）
	Remark         string `json:"remark"`                 // 备注（用户）
	Content        string `json:"content"`                // 正文（用户）
	Level          string `json:"level"`                  // 级别代码（用户选）
	LevelName      string `json:"levelName"`              // 级别名称（SDK 可自动填）
	AttachmentID   int64  `json:"attachmentId,omitempty"` // 附件 ID（上传后获得）
	AttachmentName string `json:"attachmentName"`         // 附件文件名（上传后获得）
}

// UnmarshalJSON 兼容前端表单初始 attachmentId:""。
// 空字符串/null 表示尚未上传附件，归一为零值；数字仍按原值保留。
func flexStringFromNumber(raw json.RawMessage, field string) (string, bool, error) {
	if len(raw) == 0 {
		return "", false, nil
	}
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return "", false, nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return "", false, fmt.Errorf("%s: %w", field, err)
		}
		return strings.TrimSpace(s), true, nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return "", false, fmt.Errorf("%s: 期望字符串或数字，得到 %s: %w", field, string(data), err)
	}
	if i, err := n.Int64(); err == nil {
		return strconv.FormatInt(i, 10), true, nil
	}
	f, err := n.Float64()
	if err != nil {
		return "", false, fmt.Errorf("%s: 无法解析 number %q: %w", field, n.String(), err)
	}
	if f != float64(int64(f)) {
		return "", false, fmt.Errorf("%s: 期望整数，得到 %v", field, f)
	}
	return strconv.FormatInt(int64(f), 10), true, nil
}

func (p *AddTypicalCasePayload) UnmarshalJSON(data []byte) error {
	type payloadAlias AddTypicalCasePayload
	var raw struct {
		Type         json.RawMessage `json:"type"`
		Role         json.RawMessage `json:"role"`
		Level        json.RawMessage `json:"level"`
		AttachmentID json.RawMessage `json:"attachmentId"`
		*payloadAlias
	}
	raw.payloadAlias = (*payloadAlias)(p)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok, err := flexStringFromNumber(raw.Type, "type"); err != nil {
		return err
	} else if ok {
		p.Type = v
	}
	if v, ok, err := flexStringFromNumber(raw.Role, "role"); err != nil {
		return err
	} else if ok {
		p.Role = v
	}
	if v, ok, err := flexStringFromNumber(raw.Level, "level"); err != nil {
		return err
	} else if ok {
		p.Level = v
	}
	id := bytes.TrimSpace(raw.AttachmentID)
	if len(id) == 0 {
		return nil
	}
	if bytes.Equal(id, []byte("null")) || bytes.Equal(id, []byte(`""`)) {
		p.AttachmentID = 0
		return nil
	}
	if id[0] == '"' {
		var s string
		if err := json.Unmarshal(id, &s); err != nil {
			return fmt.Errorf("attachmentId: %w", err)
		}
		if strings.TrimSpace(s) == "" {
			return nil
		}
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return fmt.Errorf("attachmentId: %w", err)
		}
		p.AttachmentID = n
		return nil
	}
	if err := json.Unmarshal(id, &p.AttachmentID); err != nil {
		return fmt.Errorf("attachmentId: %w", err)
	}
	return nil
}

// MarshalJSON 无附件时省略 attachmentId，与提交前的前端表单语义一致。
func (p AddTypicalCasePayload) MarshalJSON() ([]byte, error) {
	type payloadAlias AddTypicalCasePayload
	return json.Marshal(payloadAlias(p))
}

// TypicalCaseRecord 是已提交的典型案例记录（来自 getTypicalCase 列表接口）。
//
// 与 AddTypicalCasePayload 不同的 Go 类型：列表响应中的 type/role/level 是整数，
// 而提交请求体中是字符串。包含数字代码字段供编辑场景使用。
type TypicalCaseRecord struct {
	ID             int64  `json:"id"`             // 记录 ID
	Title          string `json:"title"`          // 标题
	Type           int    `json:"type"`           // 材料类别代码（列表返回为整数）
	TypeName       string `json:"typeName"`       // 材料类别名称
	TeacherName    string `json:"teacherName"`    // 指导教师
	PartnerName    string `json:"partnerName"`    // 合作者
	Role           int    `json:"role"`           // 角色代码（列表返回为整数）
	RoleName       string `json:"roleName"`       // 角色名称
	Remark         string `json:"remark"`         // 备注
	Content        string `json:"content"`        // 正文
	Level          int    `json:"level"`          // 级别代码（列表返回为整数）
	LevelName      string `json:"levelName"`      // 级别名称
	AttachmentID   int64  `json:"attachmentId"`   // 附件 ID
	AttachmentName string `json:"attachmentName"` // 附件文件名
	Status         int    `json:"status"`         // 审核状态（0=未审核）
	StatusName     string `json:"statusName"`     // 审核状态名称（"未审核"）
	TermID         int64  `json:"termId"`         // 学期 ID
	TermName       string `json:"termName"`       // 学期名称
	GradeName      string `json:"gradeName"`      // 年级名称
	ClassName      string `json:"className"`      // 班级名称
	StudentName    string `json:"studentName"`    // 学生姓名
	AuditRemark    string `json:"auditRemark"`    // 学校审核意见（v1.4.0 新增）
}

// TypicalCaseListResult 是 GetTypicalCaseList 的统一返回对象。
type TypicalCaseListResult struct {
	Records []TypicalCaseRecord `json:"records"`
	Page    *PageBean           `json:"page,omitempty"`
}

// parseFlexInt 解析前端可能返回为数字、数字字符串、浮点 1.0、null、空串的整数字段。
// 空/null 归零；带空格字符串会 trim；浮点仅接受整数值，非整数返回错误。
func parseFlexInt(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return 0, nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return 0, err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("期望整数，得到 %q: %w", s, err)
		}
		return n, nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return 0, fmt.Errorf("期望整数，得到 %s: %w", string(data), err)
	}
	if i, err := n.Int64(); err == nil {
		return int(i), nil
	}
	f, err := n.Float64()
	if err != nil {
		return 0, fmt.Errorf("无法解析数值 %q: %w", n.String(), err)
	}
	if f != float64(int64(f)) {
		return 0, fmt.Errorf("期望整数，得到 %v", f)
	}
	return int(f), nil
}

// parseFlexInt64 解析 attachmentId 等 int64 字段：数字/数字字符串/null/空串均兼容。
func parseFlexInt64(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return 0, nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return 0, err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("期望整数，得到 %q: %w", s, err)
		}
		return n, nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return 0, fmt.Errorf("期望整数，得到 %s: %w", string(data), err)
	}
	if i, err := n.Int64(); err == nil {
		return i, nil
	}
	f, err := n.Float64()
	if err != nil {
		return 0, fmt.Errorf("无法解析数值 %q: %w", n.String(), err)
	}
	if f != float64(int64(f)) {
		return 0, fmt.Errorf("期望整数，得到 %v", f)
	}
	return int64(f), nil
}

// UnmarshalJSON 使 TypicalCaseRecord 的 type/role/level/attachmentId 兼容 int 与 string 数字。
//
// 背景：审计报告 08 指出前端列表响应可能返回 string（如 "1"），而编辑回填时 form.type
// 原样透传，裸 int 会导致 DecodeDataList 整页失败（与 FlexBool/IntList 同类全灭）。
// 本方法接受 number、数字字符串（含空格）、浮点 1.0、null、空串，均归一为 int/int64。
func (r *TypicalCaseRecord) UnmarshalJSON(data []byte) error {
	type recordAlias TypicalCaseRecord
	var raw struct {
		Type         json.RawMessage `json:"type"`
		Role         json.RawMessage `json:"role"`
		Level        json.RawMessage `json:"level"`
		AttachmentID json.RawMessage `json:"attachmentId"`
		*recordAlias
	}
	raw.recordAlias = (*recordAlias)(r)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Type != nil {
		v, err := parseFlexInt(raw.Type)
		if err != nil {
			return fmt.Errorf("type: %w", err)
		}
		r.Type = v
	}
	if raw.Role != nil {
		v, err := parseFlexInt(raw.Role)
		if err != nil {
			return fmt.Errorf("role: %w", err)
		}
		r.Role = v
	}
	if raw.Level != nil {
		v, err := parseFlexInt(raw.Level)
		if err != nil {
			return fmt.Errorf("level: %w", err)
		}
		r.Level = v
	}
	if raw.AttachmentID != nil {
		v, err := parseFlexInt64(raw.AttachmentID)
		if err != nil {
			return fmt.Errorf("attachmentId: %w", err)
		}
		r.AttachmentID = v
	}
	return nil
}
