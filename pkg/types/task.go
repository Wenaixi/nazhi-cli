package types

import "time"

// 任务作用域常量（对应服务端 scopeType）。
const (
	ScopeClass = 1 // 班级任务
	ScopeGrade = 2 // 年段任务
	ScopeStage = 3 // 学段任务
)

// Task 是面向调用方的精简任务条目。
//
// 字段命名约定：原样的字段保持与服务端 JSON 字段名一致；需要后处理的字段使用更清晰的
// 语义名称（Submitted/NeedPic/StartDate/EndDate），由 SDK 层在 FetchTasks 中完成映射。
type Task struct {
	ID            int64     `json:"id"`            // 任务 ID（即 circleTaskId）
	Name          string    `json:"name"`          // 任务名称
	TypeName      string    `json:"typeName"`      // 类型名称
	DimensionName string    `json:"dimensionName"` // 维度名称
	Hours         float64   `json:"hours"`         // 学时
	Submitted     bool      `json:"submitted"`     // 是否已提交（来自服务端 circleTaskStatus）
	NeedPic       bool      `json:"needPic"`       // 是否需要图片（来自服务端 upPic 0/1）
	StartDate     time.Time `json:"startDate"`     // 开始日期（来自服务端 startDateStr，如 2026-01-12）
	EndDate       time.Time `json:"endDate"`       // 结束日期（来自服务端 endDateStr，如 2026-02-10）
	ScopeType     int       `json:"scopeType"`     // 作用域类型（参见 ScopeClass/ScopeGrade/ScopeStage）
	ScopeTypeName string    `json:"scopeTypeName"` // 作用域名称
}

// TaskSubmitPayload 是 addCircle 接口的完整请求体（29 字段透传）。
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
	PlayRole            string  `json:"playRole"`
	LikeSpecialty1      string  `json:"likeSpecialty1"`
	LikeSpecialty2      string  `json:"likeSpecialty2"`
	LikeSpecialty3      string  `json:"likeSpecialty3"`
}

type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
