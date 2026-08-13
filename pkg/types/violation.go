package types

// ViolationRecord 是一条违规违纪记录（来自 getViolation 接口）。
//
// 字段名和 JSON 标签以 performanceM.vue / performanceBox.vue 的真实访问方式为准。
// AttachmentID 使用指针，以保留前端 null 表示无附件的语义。
type ViolationRecord struct {
	ID            int64     `json:"id"`
	StudentName   string    `json:"studentName"`
	ClassName     string    `json:"className"`
	GradeName     string    `json:"gradeName"`
	TypeName      string    `json:"typeName"`
	Name          string    `json:"name"`
	FromTableName string    `json:"fromTableName"`
	Score         FlexFloat `json:"score"`
	AttachmentID  *int64    `json:"attachmentId"`
	GetDateStr    string    `json:"getDateStr"`
	CreatorName   string    `json:"creatorName"`
	IfShow        string    `json:"ifshow"`
}

// ViolationType 是前端违纪说明下拉列表中的一个违规事由。
type ViolationType struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ViolationListResult 是违规记录的分页结果。
type ViolationListResult struct {
	Records []ViolationRecord `json:"records"`
	Page    *PageBean         `json:"page,omitempty"`
}
