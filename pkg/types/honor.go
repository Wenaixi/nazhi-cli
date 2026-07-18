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
type HonorRecord struct {
	ID               int64  `json:"id"`
	TypeName         string `json:"typeName"`
	LevelName        string `json:"levelName"`
	Level            int    `json:"level"`
	DimensionName    string `json:"dimensionName"`
	Approved         bool   `json:"approved"`
	ApprovedName     string `json:"approvedName"`
	GetDate          string `json:"getDate"`          // 原始字符串
	EvaluationAgency string `json:"evaluationAgency"`
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
