package types

import "strings"

// 任务作用域常量（对应服务端 scopeType）。
const (
	ScopeClass = 1 // 班级任务
	ScopeGrade = 2 // 年段任务
	ScopeStage = 3 // 学段任务
)

// 承担角色常量（对应服务端 playRole 数字编码）。
const (
	PlayRoleHost            = "1" // 主持策划者
	PlayRoleMainParticipant = "2" // 主要参与者
	PlayRoleParticipant     = "3" // 参与者
)

// Task 是面向调用方的精简任务条目。
//
// 字段命名约定：原样的字段保持与服务端 JSON 字段名一致；需要后处理的字段使用更清晰的
// 语义名称（Submitted/NeedPic/StartDate/EndDate），由 SDK 层在 FetchTasks 中完成映射。
//
// v1.0.x 恢复：auditStartDate/auditEndDate（审核期判断）、score（学分）、remark（任务说明）、
// creatorName/roleName（布置者 + 角色）、creationTime/creationTimeStr（任务创建时间）、
// termId（学期 ID）、pushNum（推送次数）。
type Task struct {
	ID              int64    `json:"id"`                // 任务 ID（即 circleTaskId）
	Name            string   `json:"name"`              // 任务名称
	TypeName        string   `json:"typeName"`          // 类型名称
	DimensionName   string   `json:"dimensionName"`     // 维度名称
	Hours           float64  `json:"hours"`             // 学时
	Score           float64  `json:"score"`             // 学分（与 hours 不同）
	Remark          string   `json:"remark"`            // 任务说明（"照片加描述" 等）
	Submitted       bool     `json:"submitted"`         // 是否已提交（来自服务端 circleTaskStatus）
	NeedPic         bool     `json:"needPic"`           // 是否需要图片（来自服务端 upPic 0/1）
	CircleTaskStatus string   `json:"circleTaskStatus"` // 平台原始提交状态
	UpPic            int      `json:"upPic"`            // 平台原始图片要求：1 需要，0 不需要
	StartDate       DateOnly `json:"startDateStr"`      // 开始日期（来自服务端 startDateStr，如 2026-01-12）
	EndDate         DateOnly `json:"endDateStr"`        // 结束日期（来自服务端 endDateStr，如 2026-02-10）
	AuditStartDate  DateOnly `json:"auditStartDateStr"` // 审核开始日期
	AuditEndDate    DateOnly `json:"auditEndDateStr"`   // 审核截止日期
	CreatorName     string   `json:"creatorName"`       // 创建者（"许风华"/"管理员"）
	RoleName        string   `json:"roleName"`          // 创建者角色（"班主任"）
	CreationTime    []int    `json:"creationTime"`      // 任务创建时间（[y,m,d,h,m,s] 数组，Java LocalDateTime）
	CreationTimeStr DateOnly `json:"creationTimeStr"`   // 任务创建日期字符串（YYYY-MM-DD）
	TermID          int64    `json:"termId"`            // 学期 ID
	PushNum         int      `json:"pushNum"`           // 推送次数
	ScopeType       int      `json:"scopeType"`         // 作用域类型（参见 ScopeClass/ScopeGrade/ScopeStage）
	ScopeTypeName   string   `json:"scopeTypeName"`     // 作用域名称
}

// RefreshSubmitted 根据平台原始字段同步提交状态和图片要求。
func (t *Task) RefreshSubmitted() {
	t.Submitted = strings.Contains(t.CircleTaskStatus, "已提交") ||
		strings.Contains(t.CircleTaskStatus, "审核通过") ||
		strings.Contains(t.CircleTaskStatus, "审核中") ||
		strings.Contains(t.CircleTaskStatus, "已审核")
	t.NeedPic = t.UpPic == 1
}

// TaskSubmitPayload 是 addCircle 接口的完整请求体（30 字段透传）。
type TaskSubmitPayload struct {
	ID                  *int64  `json:"id"`
	Name                string  `json:"name"`
	HostName            string  `json:"hostName"`
	CircleDate          string  `json:"circleDate"`
	Rank                string  `json:"rank"`
	Level               string  `json:"level"`
	Content             string  `json:"content"`
	PictureList         []int64 `json:"pictureList"`
	CircleTaskID        int64   `json:"circleTaskId"`
	CircleTypeID        int64   `json:"circleTypeId"`
	DimensionID         int64   `json:"dimensionId"`
	Hours               float64 `json:"hours"`
	CircleBeginDate     string  `json:"circleBeginDate"`
	CircleEndDate       string  `json:"circleEndDate"`
	CheckResult         string  `json:"checkResult"`
	PatentType          string  `json:"patentType"`
	PatentNum           string  `json:"patentNum"`
	Address             string  `json:"address"`
	TermName            string  `json:"termName"`
	ActivityName        string  `json:"activityName"`
	SportsName          string  `json:"sportsName"`
	TeamName            string  `json:"teamName"`
	OrgName             string  `json:"orgName"`
	ResultsName         string  `json:"resultsName"`
	ObtainTime          string  `json:"obtainTime"`
	SpecialtyTechnology string  `json:"specialtyTechnology"`
	// PlayRole 承担角色（数字编码，见 PlayRoleHost / PlayRoleMainParticipant / PlayRoleParticipant 常量）。
	PlayRole       string `json:"playRole"`
	LikeSpecialty1 string `json:"likeSpecialty1"`
	LikeSpecialty2 string `json:"likeSpecialty2"`
	LikeSpecialty3 string `json:"likeSpecialty3"`
}

type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
