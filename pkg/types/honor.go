package types

import "time"

// HonorType 一种可申报的荣誉类型（来自 getHonorType 接口）。
//
// 字段裁剪原则（v1.0.0 起）：
//   - 仅保留调用方在展示/选择时需要的核心标识与级别信息
//   - Score / DimensionID / SortNo 等运营/排序字段已移除，避免冗余数据流入业务逻辑
type HonorType struct {
	ID            int64  `json:"id"`            // 荣誉类型 ID
	Name          string `json:"name"`          // 荣誉名称
	LevelName     string `json:"levelName"`     // 级别名（校/区县/市/省/国家）
	Level         int    `json:"level"`         // 级别代码（5=校, 4=区县, 3=市, 2=省, 1=国家）
	DimensionName string `json:"dimensionName"` // 维度名
}

// HonorRecord 一条已申报的荣誉记录（来自 getHonorByStudentId 接口）。
//
// 字段裁剪原则（v1.0.0 起）：
//   - 仅保留记录展示与申报回显所需的字段
//   - Score / TypeID / DimensionID 等冗余 ID 已移除（可通过 typeName 反查）
//   - StudentName / ClassName 等用户维度信息已移除，避免与 UserInfo 重复
//   - 原 int status 收敛为 bool approved（true=审核通过），并以 approvedName 保留中文文案
//   - GetDate 由字符串升级为 time.Time，调用方可直接做时间运算
type HonorRecord struct {
	ID               int64     `json:"id"`               // 荣誉记录 ID
	TypeName         string    `json:"typeName"`         // 荣誉类型名
	LevelName        string    `json:"levelName"`        // 级别名
	Level            int       `json:"level"`            // 级别代码
	DimensionName    string    `json:"dimensionName"`    // 维度名
	Approved         bool      `json:"approved"`         // 是否已通过审核（true=已通过，替代原 int status）
	ApprovedName     string    `json:"approvedName"`     // 审核状态名（替代原 statusName）
	GetDate          time.Time `json:"getDate"`          // 获得日期（替代原 get_date 字符串）
	EvaluationAgency string    `json:"evaluationAgency"` // 颁发机构
}

// AddHonorPayload 是 addHonor 接口的请求体。
type AddHonorPayload struct {
	Name                string `json:"name"`                // 荣誉名称
	TypeID              int64  `json:"typeId"`              // 荣誉类型 ID
	TypeName            string `json:"typeName"`            // 荣誉类型名
	Level               int    `json:"level"`               // 级别代码
	EvaluationAgency    string `json:"evaluationAgency"`    // 颁发机构
	GetDate             string `json:"getDate"`             // 获得日期（YYYY-MM-DD）
	CertImgAttachmentID string `json:"certImgAttachmentId"` // 证书图片附件 ID 或空
}

// HonorSelectOption 是下拉选择选项（label/value 对）。
type HonorSelectOption struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}