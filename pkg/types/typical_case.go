package types

// 典型案例角色常量（对应服务端 role 字段）。
const (
	TypicalCaseRoleHost        = "1" // 负责人
	TypicalCaseRoleParticipant = "2" // 参与者
)

// AddTypicalCasePayload 是 addTypicalCase 接口的请求体。
//
// type/role/level 在请求体中是 JSON 字符串（HAR 确认），
// 与列表响应中的整数类型不同。
type AddTypicalCasePayload struct {
	Title          string `json:"title"`          // 标题
	Type           string `json:"type"`           // 材料类别代码（"1"）
	TypeName       string `json:"typeName"`       // 材料类别名称（"研究性学习报告"）
	TeacherName    string `json:"teacherName"`    // 指导教师
	PartnerName    string `json:"partnerName"`    // 合作者
	Role           string `json:"role"`           // 角色代码（TypicalCaseRoleHost=负责人）
	RoleName       string `json:"roleName"`       // 角色名称（"负责人"）
	Remark         string `json:"remark"`         // 备注
	Content        string `json:"content"`        // 正文
	Level          string `json:"level"`          // 级别代码（"5"）
	LevelName      string `json:"levelName"`      // 级别名称（"学校"）
	AttachmentID   int64  `json:"attachmentId"`   // 附件 ID（先上传图片后获得）
	AttachmentName string `json:"attachmentName"` // 附件文件名
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
