package types

// HonorType 一种可申报的荣誉类型（来自 getHonorType 接口）。
type HonorType struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	LevelName     string `json:"levelName"`
	Level         int    `json:"level"`
	DimensionName string `json:"dimensionName"`
}

// HonorRecord 一条已申报的荣誉记录（来自 getHonorByStudentId 接口）。
//
// v2.0.0 变更：GetDate 改为 string，保留服务端原始日期格式。
//
// v1.4.0 修复：JSON tag 修正为 snake_case 以匹配 API 实际返回的字段名，
// 补充 honor_list 中前端用到的 type_id / cert_img_attachment_id / score / score_name 字段。
type HonorRecord struct {
	ID               int64  `json:"id"`
	TypeName         string `json:"type_name"`
	LevelName        string `json:"level_name"`
	Level            int    `json:"level"`
	DimensionName    string `json:"dimension_name"`
	Approved         bool   `json:"approved"`
	ApprovedName     string `json:"approved_name"`
	GetDate          string `json:"get_date"` // 原始日期字符串
	EvaluationAgency string `json:"evaluation_agency"`

	// v1.4.0 新增：前端 performanceM.vue 中实际使用的字段
	TypeID              int64  `json:"type_id,omitempty"`                // 荣誉类型 ID
	CertImgAttachmentID string `json:"cert_img_attachment_id,omitempty"` // 证书图片附件 ID
	Score               int    `json:"score,omitempty"`                  // 分数
	ScoreName           string `json:"score_name,omitempty"`             // 分数描述
}

// HonorListResult 是 GetHonorList 的统一返回对象。
type HonorListResult struct {
	Records []HonorRecord `json:"records"`
	Page    *PageBean     `json:"page,omitempty"`
}

// AddHonorPayload 是 addHonor 接口的请求体。
type AddHonorPayload struct {
	Name                string `json:"name"`
	TypeID              int64  `json:"typeId"`
	TypeName            string `json:"typeName"`
	Level               int    `json:"level"`
	EvaluationAgency    string `json:"evaluationAgency"`
	GetDate             string `json:"getDate"`
	CertImgAttachmentID string `json:"certImgAttachmentId"`
}

// HonorSelectOption 是下拉选择选项。
type HonorSelectOption struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}
