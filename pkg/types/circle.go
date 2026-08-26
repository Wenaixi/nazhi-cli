package types

// PageBean 目标平台通用分页信息。
type PageBean struct {
	PageNo    int `json:"pageNo"`
	PageSize  int `json:"pageSize"`
	TotalNum  int `json:"totalNum"`
	TotalPage int `json:"totalPage"`
}

// CircleRecord 一条已提交的写实记录（来自 getStudentCircle 接口）。
//
// 平台字段命名混用（对照前端源码确认）：
//   - 活动/任务元数据多为 snake_case：host_name、type_name、circle_task_name …
//   - UI 相关字段为 camelCase：imgList、imgPreViewList、commentList、likeStatus、
//     ifMySelf、creationTimeStr、showName、imgPath、studentId
//
// JSON tag 必须以平台真实键名为准，禁止假设「全 snake 或全 camel」。
// Go 字段名保持驼峰惯例。日期字段为 string（保留服务端原始格式）；
// Level/CheckResult 为 int；camelCase 与 snake_case 混用以真实 API 为准。
//
// 明确不建模（18 轮审计裁决，HAR .claude/我分别执行了….har:238/:494 实证存在于服务端响应、
// 但前端写实卡片零消费）：score（计分）、term_id、last_auditor、last_audit_time、last_auditor_name
// 五个审核/计分元数据字段。维持不建模符合「不建模未展示字段」纪律——扩字段反增 DecodeDataList
// 全灭风险面（FlexFloat/int64 对形态违约零容错）；需要原始字节请走 *JSON 透传方法族。
//
// ShowName 非平台字段：showName 是前端自行拼接的派生字段（managementRightBottom.vue:678 把活动/
// 竞赛字段拼 HTML 后赋值），服务端 getStudentCircle 不返回该键（HAR 实证）。SDK 解码恒为零值，
// 需要展示名请按前端 :555-678 拼接逻辑自行处理。
type CircleRecord struct {
	// 基础字段
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TypeName string `json:"type_name"`
	// Approved：业务以 Status 为准；解码兼容 bool/0/1（见 FlexBool）。
	Approved       FlexBool      `json:"approved"`
	CircleDate     string        `json:"circle_date"`
	// Hours 为数值（平台返回 number，前端模板字符串拼接展示「X小时」）；
	// 空值语义请用 Hours==0 判定，不要做字符串空串判断（19 轮审计 P2-1 披露）。
	Hours          float64       `json:"hours"`
	ImgList        []CircleImage `json:"imgList"`
	ImgPreViewList []string      `json:"imgPreViewList"`
	Remark         string        `json:"remark"`

	// 类型与状态编号（19 轮审计 P2-2 披露语义，防历史误读复发）：
	// Type = 前端 tab：1 公示 / 2 教师写实 / 3 我发布 / 4 被撤回（getStudentCircle 的 type 参数）。
	// Status：0 已发布（可编辑删除）/ 1 已锁定（不可编辑）/ 2 被撤回（配 auditRemark 红字原因）。
	Type   int `json:"type,omitempty"`
	Status int `json:"status,omitempty"`

	// 活动/竞赛相关字段（API 返回 snake_case）
	HostName            string `json:"host_name,omitempty"`
	Rank                string `json:"rank,omitempty"`
	Level               int    `json:"level,omitempty"`
	CircleBeginDate     string `json:"circle_begin_date,omitempty"`
	CircleEndDate       string `json:"circle_end_date,omitempty"`
	CheckResult         int    `json:"check_result,omitempty"`
	PatentType          string `json:"patent_type,omitempty"`
	PatentNum           string `json:"patent_num,omitempty"`
	Address             string `json:"address,omitempty"`
	TermName            string `json:"term_name,omitempty"`
	ActivityName        string `json:"activity_name,omitempty"`
	SportsName          string `json:"sports_name,omitempty"`
	TeamName            string `json:"team_name,omitempty"`
	OrgName             string `json:"org_name,omitempty"`
	ResultsName         string `json:"results_name,omitempty"`
	ObtainTime          string `json:"obtain_time,omitempty"`
	SpecialtyTechnology string `json:"specialty_technology,omitempty"`
	// PlayRole：列表 API 常为 number，提交表单为 string；见 PlayRoleCode。
	PlayRole       PlayRoleCode `json:"play_role,omitempty"`
	LikeSpecialty1 string       `json:"like_specialty1,omitempty"`
	LikeSpecialty2 string       `json:"like_specialty2,omitempty"`
	LikeSpecialty3 string       `json:"like_specialty3,omitempty"`

	// 元数据字段（creationTimeStr/showName 为 camelCase）
	OperatorName    string `json:"operator_name,omitempty"`
	CreationTimeStr string `json:"creationTimeStr,omitempty"`
	CircleTaskName  string `json:"circle_task_name,omitempty"`
	ShowName        string `json:"showName,omitempty"`

	// 学生/创建者信息
	Creator      int64  `json:"creator,omitempty"`
	StudentId    int64  `json:"studentId,omitempty"`
	StudentNum   string `json:"student_num,omitempty"`
	ClassName    string `json:"class_name,omitempty"`
	GradeName    string `json:"grade_name,omitempty"`
	CreatorName  string `json:"creator_name,omitempty"`
	CreationTime int64  `json:"creation_time,omitempty"`

	// 范围/状态信息
	ScopeType      int    `json:"scope_type,omitempty"`
	ScopeTypeName  string `json:"scope_type_name,omitempty"`
	StateType      int    `json:"state_type,omitempty"`
	CircleTypeId   int64  `json:"circle_type_id,omitempty"`
	CircleTaskId   int64  `json:"circle_task_id,omitempty"`
	RoleId         int64  `json:"role_id,omitempty"`
	RoleName       string `json:"role_name,omitempty"`
	PushStatus     int    `json:"push_status,omitempty"`
	PushNum        int    `json:"push_num,omitempty"`
	OperatorId     int64  `json:"operator_id,omitempty"`
	ClassID        int64  `json:"class_id,omitempty"`
	SchoolID       int64  `json:"school_id,omitempty"`
	GradeID        int64  `json:"grade_id,omitempty"`
	StartDate      string `json:"start_date,omitempty"`
	EndDate        string `json:"end_date,omitempty"`
	AuditStartDate string `json:"audit_start_date,omitempty"`
	AuditEndDate   string `json:"audit_end_date,omitempty"`
	AreaId         int64  `json:"area_id,omitempty"`
	AreaTaskId     int64  `json:"area_task_id,omitempty"`
	DimensionId    int64  `json:"dimension_id,omitempty"`
	ShowType       int    `json:"show_type,omitempty"`
	ScoreNum       int    `json:"score_num,omitempty"`
	UpPic          int    `json:"up_pic,omitempty"`

	// 点赞信息
	LikeList []any `json:"likeList,omitempty"` // API 实际键名为 camelCase（HAR 实证），非 snake

	// 状态字段（ifMySelf 为数字 0/1，前端 item.ifMySelf==1；
	// likeStatus / auditRemark 为 camelCase，见 managementRightBottom.vue）
	IfMySelf    int      `json:"ifMySelf,omitempty"`
	AuditRemark string   `json:"auditRemark,omitempty"`
	LikeStatus  FlexBool `json:"likeStatus,omitempty"`

	// 评论列表（camelCase）
	CommentList []Comment `json:"commentList,omitempty"`
}

// Comment 写实记录关联的评论。
type Comment struct {
	ID              int64  `json:"id"`
	Content         string `json:"content"`
	Commentator     int64  `json:"commentator"`
	CommentatorName string `json:"commentator_name"`
	CommentatorType int    `json:"commentator_type"`
	CommentTime     string `json:"comment_time"`
}

// CircleImage 写实记录关联的图片附件。
// imgPath 为 camelCase（前端 item2.imgPath）；其余 id 类字段为 snake_case。
type CircleImage struct {
	ID           int64  `json:"id"`
	CircleID     int64  `json:"circle_id"`
	ClassID      int64  `json:"class_id"`
	TaskID       int64  `json:"task_id"`
	AttachmentID int64  `json:"attachment_id"`
	ImgPath      string `json:"imgPath"`
}
