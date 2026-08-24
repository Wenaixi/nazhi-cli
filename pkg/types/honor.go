package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// HonorType 一种可申报的荣誉类型（来自 getHonorType 接口）。
//
// 前端 performanceM.vue 德育说明表列使用 snake_case：
// dimension_name / level_name / score（非 camelCase）。
//
// 实测 getHonorType 的 score 为展示文案（如 "分数：+5.0"），不是 JSON number。
// 误标为 int 会导致整页 DecodeDataList 失败（与 admissionDate 同类）。
type HonorType struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LevelName     string `json:"level_name"`
	Level         int    `json:"level"`
	DimensionName string `json:"dimension_name"`
	Score         string `json:"score,omitempty"` // 展示分值文案，如 "分数：+5.0"
}

// HonorRecord 一条已申报的荣誉记录（来自 getHonorByStudentId 接口）。
//
// GetDate 为 string，保留服务端原始日期格式。JSON tag 为 snake_case 以匹配 API 实际返回的字段名。
// Status 为整型审核状态（前端 scope.row.status != 1 控制编辑/删除）；
// Approved 兼容 bool/0/1 解码。
//
// 设计取舍：服务端下发的 HonorRecord 可能携带 dimension_id / auditor_name 等
// 报告单展示用只读字段，前端 performanceM.vue / performanceBox.vue 荣誉表格仅展示
// type_name / level_name / score / ifshow / statusName 等，未用于提交逻辑或状态分支判断。
// 为保持类型精简与可维护性，本结构体暂未映射这些只读展示字段，按需扩展。
type HonorRecord struct {
	ID               int64    `json:"id"`
	TypeName         string   `json:"type_name"`
	LevelName        string   `json:"level_name"`
	Level            int      `json:"level"`
	DimensionName    string   `json:"dimension_name"`
	Approved         FlexBool `json:"approved"`   // 兼容 bool/0/1；业务以 Status 为准
	Status           int      `json:"status"`     // 审核状态码：0 未审 / 1 通过 等（前端编辑按钮用）
	ApprovedName     string   `json:"statusName"` // 审核状态名称（API 返回 statusName）
	GetDate          string   `json:"get_date"`   // 原始日期字符串
	EvaluationAgency string   `json:"evaluation_agency"`

	// 前端 performanceM.vue 中实际使用的字段
	TypeID              int64  `json:"type_id,omitempty"`                // 荣誉类型 ID
	CertImgAttachmentID string `json:"cert_img_attachment_id,omitempty"` // 证书图片附件 ID
	// Score：列表实测 JSON number（常为 4.0）；禁止 int（encoding/json 拒 4.0→int）。
	Score     float64 `json:"score,omitempty"`
	ScoreName string  `json:"score_name,omitempty"` // 分数描述

	// 前端表格列使用的字段
	IfShow      string `json:"ifshow,omitempty"`       // 是否报告单展示
	StudentName string `json:"student_name,omitempty"` // 学生姓名
	ClassName   string `json:"class_name,omitempty"`   // 班级名称
}

// HonorListResult 是 GetHonorList 的统一返回对象。
type HonorListResult struct {
	Records []HonorRecord `json:"records"`
	Page    *PageBean     `json:"page,omitempty"`
}

// AddHonorPayload 是 addHonor 接口的请求体。
//
// 用户输入（对齐 performanceM.vue v-model）：TypeID、Level、EvaluationAgency、GetDate、证书图。
// SDK 自动：TypeName（反查）、Name（回落 TypeName）、Score（默认 0，前端 form 无输入）。
// 调用方一般不必填 TypeName/Name/Score。
type AddHonorPayload struct {
	Name                string `json:"name"`
	TypeID              int64  `json:"typeId"`
	TypeName            string `json:"typeName"`
	Level               int    `json:"level"`
	EvaluationAgency    string `json:"evaluationAgency"`
	GetDate             string `json:"getDate"`
	CertImgAttachmentID string `json:"certImgAttachmentId"`
	// Score 分值。前端 form 默认 0 且无 v-model；零值也会序列化进请求体。
	Score int `json:"score"`
}

// UnmarshalJSON 兼容前端上传成功后返回的 number 类型 certImgAttachmentId，
// 同时保留调用方传入的字符串形式。
func (p *AddHonorPayload) UnmarshalJSON(data []byte) error {
	type payloadAlias AddHonorPayload
	var raw struct {
		*payloadAlias
		CertImgAttachmentID json.RawMessage `json:"certImgAttachmentId"`
	}
	raw.payloadAlias = (*payloadAlias)(p)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	value := bytes.TrimSpace(raw.CertImgAttachmentID)
	if len(value) == 0 {
		return nil
	}
	if bytes.Equal(value, []byte("null")) {
		p.CertImgAttachmentID = ""
		return nil
	}
	var text string
	if value[0] == '"' {
		if err := json.Unmarshal(value, &text); err != nil {
			return fmt.Errorf("certImgAttachmentId: %w", err)
		}
		p.CertImgAttachmentID = text
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(value, &number); err != nil {
		return fmt.Errorf("certImgAttachmentId: 期望字符串或数字: %w", err)
	}
	if _, err := strconv.ParseInt(number.String(), 10, 64); err != nil {
		return fmt.Errorf("certImgAttachmentId: 非法数字 %q: %w", number.String(), err)
	}
	p.CertImgAttachmentID = number.String()
	return nil
}

// HonorSelectOption 是下拉选择选项。
type HonorSelectOption struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}
