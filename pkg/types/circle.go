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
// v1.3.0 扩展：补齐前端所有原始字段。
// v2.0.0 变更：CircleDate 改为 string，保留服务端原始日期格式。
//
//	Level/CheckResult 改为 int，与服务端返回类型一致。
//
// v1.4.0 修复：JSON tag 修正为 snake_case 以匹配 API 实际返回的字段名（前端的 getStudentCircle
// 响应使用下划线命名），Go 字段名保持驼峰（Go 惯例）。
type CircleRecord struct {
	// 基础字段
	ID             int64         `json:"id"`
	Name           string        `json:"name"`
	Content        string        `json:"content"`
	TypeName       string        `json:"type_name"`
	Approved       bool          `json:"approved"`
	CircleDate     string        `json:"circle_date"`
	Hours          float64       `json:"hours"`
	ImgList        []CircleImage `json:"img_list"`
	ImgPreViewList []string      `json:"img_pre_view_list"`
	Remark         string        `json:"remark"`

	// 类型与状态编号
	Type   int `json:"type,omitempty"`
	Status int `json:"status,omitempty"`

	// 活动/竞赛相关字段（API 返回 snake_case，Go 字段名保持驼峰）
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
	PlayRole            string `json:"play_role,omitempty"`
	LikeSpecialty1      string `json:"like_specialty1,omitempty"`
	LikeSpecialty2      string `json:"like_specialty2,omitempty"`
	LikeSpecialty3      string `json:"like_specialty3,omitempty"`

	// 元数据字段
	OperatorName    string `json:"operator_name,omitempty"`
	CreationTimeStr string `json:"creation_time_str,omitempty"`
	CircleTaskName  string `json:"circle_task_name,omitempty"`
	ShowName        string `json:"show_name,omitempty"`

	// 学生/创建者信息（这些字段 API 已用 snake_case）
	Creator      int64  `json:"creator,omitempty"`
	StudentId    int64  `json:"student_id,omitempty"`
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
	LikeList []any `json:"like_list,omitempty"`

	// 状态字段
	IsMySelf    bool   `json:"is_my_self,omitempty"`
	AuditRemark string `json:"audit_remark,omitempty"`
	LikeStatus  bool   `json:"like_status,omitempty"`

	// 评论列表
	CommentList []Comment `json:"comment_list,omitempty"`
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
type CircleImage struct {
	ID           int64  `json:"id"`
	CircleID     int64  `json:"circle_id"`
	ClassID      int64  `json:"class_id"`
	TaskID       int64  `json:"task_id"`
	AttachmentID int64  `json:"attachment_id"`
	ImgPath      string `json:"img_path"`
}
