package types

// ViolationRecord 违规违纪记录（来自 getViolation 接口）。
type ViolationRecord struct {
	ID             int64     `json:"id"`             // 记录 ID
	StudentName    string    `json:"studentName"`    // 学生姓名
	ClassName      string    `json:"className"`      // 班级名称
	GradeName      string    `json:"gradeName"`      // 年级名称
	TypeName       string    `json:"typeName"`       // 违规违纪事由
	Name           string    `json:"name"`           // 违规违纪详情
	FromTableName  string    `json:"fromTableName"`  // 级别
	Score          float64   `json:"score"`          // 扣分
	AttachmentID   *int64    `json:"attachmentId"` // 附件 ID
	GetDateStr     string    `json:"getDateStr"`     // 获得时间
	CreatorName    string    `json:"creatorName"`    // 操作人
	IfShow         string    `json:"ifshow"`         // 是否报告单展示
	CreateTime     string `json:"createTime"`     // 创建时间（原始字符串）
}

// ViolationType 违规违纪类型（来自 getViolationType 接口）。
type ViolationType struct {
	ID   int64  `json:"id"`   // 类型 ID
	Name string `json:"name"` // 类型名称
}
