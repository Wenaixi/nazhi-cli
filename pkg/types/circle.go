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
//          Level/CheckResult 改为 int，与服务端返回类型一致。
type CircleRecord struct {
	// 基础字段
	ID             int64         `json:"id"`
	Name           string        `json:"name"`
	Content        string        `json:"content"`
	TypeName       string        `json:"typeName"`
	Approved       bool          `json:"approved"`
	CircleDate     string        `json:"circleDate"`
	Hours          float64       `json:"hours"`
	ImgList        []CircleImage `json:"imgList"`
	ImgPreViewList []string      `json:"imgPreViewList"`
	Remark         string        `json:"remark"`

	// 活动/竞赛相关字段
	HostName            string `json:"hostName,omitempty"`
	Rank                string `json:"rank,omitempty"`
	Level               int    `json:"level,omitempty"`
	CircleBeginDate     string `json:"circleBeginDate,omitempty"`
	CircleEndDate       string `json:"circleEndDate,omitempty"`
	CheckResult         int    `json:"checkResult,omitempty"`
	PatentType          string `json:"patentType,omitempty"`
	PatentNum           string `json:"patentNum,omitempty"`
	Address             string `json:"address,omitempty"`
	TermName            string `json:"termName,omitempty"`
	ActivityName        string `json:"activityName,omitempty"`
	SportsName          string `json:"sportsName,omitempty"`
	TeamName            string `json:"teamName,omitempty"`
	OrgName             string `json:"orgName,omitempty"`
	ResultsName         string `json:"resultsName,omitempty"`
	ObtainTime          string `json:"obtainTime,omitempty"`
	SpecialtyTechnology string `json:"specialtyTechnology,omitempty"`
	PlayRole            string `json:"playRole,omitempty"`
	LikeSpecialty1      string `json:"likeSpecialty1,omitempty"`
	LikeSpecialty2      string `json:"likeSpecialty2,omitempty"`
	LikeSpecialty3      string `json:"likeSpecialty3,omitempty"`

	// 元数据字段
	OperatorName   string `json:"operatorName,omitempty"`
	CreationTimeStr string `json:"creationTimeStr,omitempty"`
	CircleTaskName string `json:"circleTaskName,omitempty"`
	ShowName       string `json:"showName,omitempty"`

	// 状态字段
	IsMySelf   bool   `json:"isMySelf,omitempty"`
	AuditRemark string `json:"auditRemark,omitempty"`
	LikeStatus  bool   `json:"likeStatus,omitempty"`

	// 评论列表
	CommentList []Comment `json:"commentList,omitempty"`
}

// Comment 写实记录关联的评论。
type Comment struct {
	ID               int64  `json:"id"`
	Content          string `json:"content"`
	Commentator      int64  `json:"commentator"`
	CommentatorName  string `json:"commentatorName"`
	CommentatorType  int    `json:"commentatorType"`
	CommentTime      string `json:"commentTime"`
}

// CircleImage 写实记录关联的图片附件。
type CircleImage struct {
	ID           int64  `json:"id"`
	CircleID     int64  `json:"circle_id"`
	ClassID      int64  `json:"class_id"`
	TaskID       int64  `json:"task_id"`
	AttachmentID int64  `json:"attachment_id"`
	ImgPath      string `json:"imgPath"`
}
