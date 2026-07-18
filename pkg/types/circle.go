package types

import "time"

// PageBean 目标平台通用分页信息。
type PageBean struct {
	PageNo    int `json:"pageNo"`
	PageSize  int `json:"pageSize"`
	TotalNum  int `json:"totalNum"`
	TotalPage int `json:"totalPage"`
}

// CircleRecord 一条已提交的写实记录（来自 getStudentCircle 接口）。
//
// v1.3.0 扩展：补齐前端所有原始字段，保留向后兼容（新增字段均使用 omitempty）。
type CircleRecord struct {
	// 基础字段（v1.0.0 已有）
	ID             int64         `json:"id"`             // 写实记录主键
	Name           string        `json:"name"`           // 写实标题
	Content        string        `json:"content"`        // 写实正文
	TypeName       string        `json:"typeName"`       // 类型名（替代原 type_name）
	Approved       bool          `json:"approved"`       // 是否已通过审核（true=已通过，替代原 int status）
	CircleDate     time.Time     `json:"circleDate"`     // 写实发生日期（替代原 circle_date 字符串）
	Hours          float64       `json:"hours"`          // 实践时长（小时）
	ImgList        []CircleImage `json:"imgList"`        // 关联图片附件列表
	ImgPreViewList []string      `json:"imgPreViewList"` // 图片预览 URL 列表（服务端直接返回的可访问链接）
	Remark         string        `json:"remark"`         // 备注

	// v1.3.0 新增：前端 getStudentCircle 响应的全部原始字段
	// 所有新字段使用 omitempty，零值/null 时不在 JSON 输出中出现。

	// 活动/竞赛相关字段
	HostName            string `json:"hostName,omitempty"`            // 主办单位
	Rank                string `json:"rank,omitempty"`                // 名次或等级
	Level               string `json:"level,omitempty"`               // 级别代码（1=国家, 2=省, 3=地区/市, 4=区/县/街道/社区, 5=校, 6=年段）
	CircleBeginDate     string `json:"circleBeginDate,omitempty"`     // 开始时间
	CircleEndDate       string `json:"circleEndDate,omitempty"`       // 结束时间
	CheckResult         string `json:"checkResult,omitempty"`       // 考核情况（1=优, 2=良, 3=合格, 4=差）
	PatentType          string `json:"patentType,omitempty"`          // 专利类型
	PatentNum           string `json:"patentNum,omitempty"`           // 专利号
	Address             string `json:"address,omitempty"`             // 活动地点
	TermName            string `json:"termName,omitempty"`            // 届数
	ActivityName        string `json:"activityName,omitempty"`        // 活动名称
	SportsName          string `json:"sportsName,omitempty"`          // 体育项目名称
	TeamName            string `json:"teamName,omitempty"`             // 团队名称
	OrgName             string `json:"orgName,omitempty"`              // 组织单位
	ResultsName         string `json:"resultsName,omitempty"`         // 成果名称
	ObtainTime          string `json:"obtainTime,omitempty"`            // 获得时间
	SpecialtyTechnology string `json:"specialtyTechnology,omitempty"`   // 特长技术
	PlayRole            string `json:"playRole,omitempty"`            // 承担角色（1=主持策划者, 2=主要参与者, 3=参与者）
	LikeSpecialty1      string `json:"likeSpecialty1,omitempty"`      // 爱好1
	LikeSpecialty2      string `json:"likeSpecialty2,omitempty"`      // 爱好2
	LikeSpecialty3      string `json:"likeSpecialty3,omitempty"`      // 爱好3

	// 元数据字段
	OperatorName   string `json:"operatorName,omitempty"`   // 操作人名称
	CreationTimeStr string `json:"creationTimeStr,omitempty"` // 创建时间字符串
	CircleTaskName string `json:"circleTaskName,omitempty"` // 写实任务名称
	ShowName       string `json:"showName,omitempty"`       // 展示名称

	// 状态字段
	IsMySelf   bool `json:"isMySelf,omitempty"`   // 是否是自己发布的（1=是）
	AuditRemark string `json:"auditRemark,omitempty"` // 撤回原因/审核备注
	LikeStatus  bool `json:"likeStatus,omitempty"`  // 是否已点赞

	// 评论列表（前端展示用）
	CommentList []Comment `json:"commentList,omitempty"` // 评论列表
}

// Comment 写实记录关联的评论。
type Comment struct {
	ID               int64  `json:"id"`               // 评论 ID
	Content          string `json:"content"`          // 评论内容
	Commentator      int64  `json:"commentator"`      // 评论者 ID
	CommentatorName  string `json:"commentatorName"`  // 评论者名称
	CommentatorType  int    `json:"commentatorType"`  // 评论者类型
	CommentTime      string `json:"commentTime"`      // 评论时间
}

// CircleImage 写实记录关联的图片附件。
type CircleImage struct {
	ID           int64  `json:"id"`            // 图片记录主键
	CircleID     int64  `json:"circle_id"`     // 关联的写实记录 ID
	ClassID      int64  `json:"class_id"`      // 班级 ID
	TaskID       int64  `json:"task_id"`       // 关联的任务 ID
	AttachmentID int64  `json:"attachment_id"` // 附件 ID（用于查询/下载图片）
	ImgPath      string `json:"imgPath"`       // 图片扩展名（如 .jpg）
}
