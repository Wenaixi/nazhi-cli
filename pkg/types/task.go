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

// TaskSubmitInput 是公开给 SDK 调用方的最小任务提交输入。
//
// 设计目标：调用方只提供真正需要人工决策的字段；其余 30 字段 payload 由 SDK
// 内部根据 taskId 元数据、用户资料和上传结果自动组装。
type TaskSubmitInput struct {
	TaskID     int64    // 必填：任务 ID
	Content    string   // 必填：心得/感悟
	ImagePaths []string // 可选：本地图片路径列表
	PlayRole   string   // 可选：默认空串，显式传入时覆盖
	Address    string   // 可选：为空时默认 schoolName，允许调用方覆盖
	Level      string   // 可选：为空时默认校级（5），允许调用方覆盖
}

// Validate 校验最小任务提交输入。
func (in TaskSubmitInput) Validate() error {
	if in.TaskID <= 0 {
		return ErrTaskInputTaskIDRequired
	}
	if strings.TrimSpace(in.Content) == "" {
		return ErrTaskInputContentRequired
	}
	return nil
}

// TaskAddCirclePayload 是 SDK 内部使用的 addCircle 完整请求体（30 字段透传）。
//
// 不再作为公开调用契约，仅用于 SDK 内部向真实接口提交请求。
type TaskAddCirclePayload struct {
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

// TaskSubmitPayload 兼容旧调用方，已废弃。
// Deprecated: 请改用 TaskSubmitInput。仅在迁移期保留，后续移除。
type TaskSubmitPayload = TaskAddCirclePayload

// TaskResult 是 addCircle 的业务返回摘要。
type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// TaskCircleTypeInfo 是 getCircleTypeByTaskId 返回的任务提交元数据。
//
// 真实网页在提交 addCircle 前会先请求该接口，拿到 circleTypeId / dimensionId /
// hours / remark 等字段，再拼最终提交 payload。SDK 暴露该类型，避免调用方手工猜测。
type TaskCircleTypeInfo struct {
	TaskName      string  `json:"task_name"`
	CircleTypeID  int64   `json:"circle_type_id"`
	Hours         float64 `json:"hours"`
	TypeName      string  `json:"type_name"`
	DimensionID   int64   `json:"dimension_id"`
	DimensionName string  `json:"dimension_name"`
	TaskID        int64   `json:"task_id"`
	Remark        string  `json:"remark"`
	Type          int     `json:"type"`
}

var (
	ErrTaskInputTaskIDRequired  = taskInputError("taskId 为必填且必须 > 0")
	ErrTaskInputContentRequired = taskInputError("content 为必填")
)

type taskInputError string

func (e taskInputError) Error() string { return string(e) }
