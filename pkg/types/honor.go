package types

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// CertImgAttachmentID 入站展示字段：前端 performanceM.vue:25 将 scope.row.cert_img_attachment_id
	// 直接作 <img :src>（不拼接 getImg?id=），故值形态是完整图片 URL 字符串而非附件 ID，建模为 string。
	// 18 轮对抗复核：勿改 int64——URL 字符串会让 DecodeDataList 整页失败；若未来 HAR 证明平台
	// 返回数字 ID，应改 FlexString 兼容层。
	CertImgAttachmentID string `json:"cert_img_attachment_id,omitempty"`
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
	// Name：前端 addHonor 表单八键不含 name（performanceM.vue:211-220），
	// 空 Name 不序列化（omitempty）；保留字段仅为兼容旧调用方显式传入。
	Name             string `json:"name,omitempty"`
	TypeID           int64  `json:"typeId"`
	TypeName         string `json:"typeName"`
	Level            int    `json:"level"`
	EvaluationAgency string `json:"evaluationAgency"`
	// GetDate：原始日期字符串透传。前端 el-date-picker 无 value-format（performanceM.vue:106/176），
	// JSON.stringify 后实际提交 ISO 8601 UTC 时间戳（如 "2026-06-30T16:00:00.000Z"），
	// 列表回填 get_date 亦为带时区完整时间戳形态；纯日期（"2026-06-30"）是否被服务端接受以服务端裁决为准。
	GetDate string `json:"getDate"`
	// CertImgAttachmentID：出站对齐前端裸 number（performanceM.vue:576 赋 returnData.id），
	// 无附件时 omitempty 省略键；入站经 UnmarshalJSON 兼容 number/数字字符串/空串/null。
	CertImgAttachmentID int64 `json:"certImgAttachmentId,omitempty"`
	// Score 分值。前端 form 默认 0 且无 v-model；零值也会序列化进请求体。
	// float64：入站兼容列表记录常见的 4.0 字面量（encoding/json 拒 4.0→int），
	// 出站零值序列化仍为 "score":0，与前端 form 默认（JS number 0）数值等价。
	Score float64 `json:"score"`
}

// UnmarshalJSON 兼容前端上传成功后返回的 number 类型 certImgAttachmentId，
// 以及历史调用方/编辑回填可能出现的数字字符串、空串、null（复用 parseFlexInt64）。
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
	if raw.CertImgAttachmentID != nil {
		v, err := parseFlexInt64(bytes.TrimSpace(raw.CertImgAttachmentID))
		if err != nil {
			return fmt.Errorf("certImgAttachmentId: %w", err)
		}
		p.CertImgAttachmentID = v
	}
	return nil
}

// HonorSelectOption 是下拉选择选项。
type HonorSelectOption struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}
