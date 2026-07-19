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
// v2.0.0 变更：时间字段改为 string，保留服务端原始日期格式（如 "2026-01-12"）。
type Task struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	TypeName         string  `json:"typeName"`
	DimensionName    string  `json:"dimensionName"`
	Hours            float64 `json:"hours"`
	Score            float64 `json:"score"`
	Remark           string  `json:"remark"`
	CircleTaskStatus string  `json:"circleTaskStatus"`
	Submitted        bool    `json:"submitted"`
	NeedPic          bool    `json:"needPic"`
	StartDate        string  `json:"startDateStr"` // 字符串，如 "2026-01-12"
	EndDate          string  `json:"endDateStr"`   // 字符串，如 "2026-02-10"
	AuditStartDate   string  `json:"auditStartDateStr"`
	AuditEndDate     string  `json:"auditEndDateStr"`
	CreatorName      string  `json:"creatorName"`
	RoleName         string  `json:"roleName"`
	CreationTime     []int   `json:"creationTime"`
	CreationTimeStr  string  `json:"creationTimeStr"` // 字符串
	TermID           int64   `json:"termId"`
	PushNum          int     `json:"pushNum"`
	ScopeType        int     `json:"scopeType"`
	ScopeTypeName    string  `json:"scopeTypeName"`

	// 扩展字段
	SchoolID          int64   `json:"schoolId,omitempty"`
	CircleTypeID      int64   `json:"circleTypeId,omitempty"`
	Creator           int64   `json:"creator,omitempty"`
	Modifier          *int64  `json:"modifier,omitempty"`
	ModifyTime        []int   `json:"modifyTime,omitempty"`
	RoleID            int64   `json:"roleId,omitempty"`
	AuditorSubjectID  *int64  `json:"auditorSubjectId,omitempty"`
	StateType         int     `json:"stateType,omitempty"`
	AreaID            int64   `json:"areaId,omitempty"`
	AreaTaskID        int64   `json:"areaTaskId,omitempty"`
	UpPic             int     `json:"upPic,omitempty"`
	EvaluatedNumber   *int    `json:"evaluatedNumber,omitempty"`
	UnEvaluatedNumber *int    `json:"unEvaluatedNumber,omitempty"`
	UnsubmittedNumber *int    `json:"unsubmittedNumber,omitempty"`
	SubmitNumber      int     `json:"submitNumber,omitempty"`
	PictureList       []int64 `json:"pictureList,omitempty"`
	ClassID           *int64  `json:"classId,omitempty"`
	GradeID           *int64  `json:"gradeId,omitempty"`
}

// TaskInput 定义任务提交/编辑输入的公共接口，用于提取公共 payload 构建逻辑。
type TaskInput interface {
	Validate() error
	GetID() *int64
	GetTaskID() int64
	GetContent() string
	GetImagePaths() []string
	GetImageIDs() []int64
	GetPlayRole() string
	GetAddress() string
	GetLevel() string
	GetName() string
	GetHostName() string
	GetCircleDate() string
	GetRank() string
	GetActivityName() string
	GetSportsName() string
	GetTeamName() string
	GetOrgName() string
	GetResultsName() string
	GetObtainTime() string
	GetSpecialtyTechnology() string
	GetLikeSpecialty1() string
	GetLikeSpecialty2() string
	GetLikeSpecialty3() string
}

// TaskSubmitInput 是公开给 SDK 调用方的最小任务提交输入。
type TaskSubmitInput struct {
	TaskID     int64
	Content    string
	ImagePaths []string
	ImageIDs   []int64
	PlayRole   string
	Address    string
	Level      string

	Name                string
	HostName            string
	CircleDate          string
	Rank                string
	ActivityName        string
	SportsName          string
	TeamName            string
	OrgName             string
	ResultsName         string
	ObtainTime          string
	SpecialtyTechnology string
	LikeSpecialty1      string
	LikeSpecialty2      string
	LikeSpecialty3      string
}

func (in TaskSubmitInput) Validate() error {
	if in.TaskID <= 0 {
		return ErrTaskInputTaskIDRequired
	}
	if strings.TrimSpace(in.Content) == "" {
		return ErrTaskInputContentRequired
	}
	return nil
}

// TaskInput 接口实现：TaskSubmitInput 没有 ID 字段，新增记录时 ID 为 nil。
func (in TaskSubmitInput) GetID() *int64                  { return nil }
func (in TaskSubmitInput) GetTaskID() int64               { return in.TaskID }
func (in TaskSubmitInput) GetContent() string             { return in.Content }
func (in TaskSubmitInput) GetImagePaths() []string        { return in.ImagePaths }
func (in TaskSubmitInput) GetImageIDs() []int64           { return in.ImageIDs }
func (in TaskSubmitInput) GetPlayRole() string            { return in.PlayRole }
func (in TaskSubmitInput) GetAddress() string             { return in.Address }
func (in TaskSubmitInput) GetLevel() string               { return in.Level }
func (in TaskSubmitInput) GetName() string                { return in.Name }
func (in TaskSubmitInput) GetHostName() string            { return in.HostName }
func (in TaskSubmitInput) GetCircleDate() string          { return in.CircleDate }
func (in TaskSubmitInput) GetRank() string                { return in.Rank }
func (in TaskSubmitInput) GetActivityName() string        { return in.ActivityName }
func (in TaskSubmitInput) GetSportsName() string          { return in.SportsName }
func (in TaskSubmitInput) GetTeamName() string            { return in.TeamName }
func (in TaskSubmitInput) GetOrgName() string             { return in.OrgName }
func (in TaskSubmitInput) GetResultsName() string         { return in.ResultsName }
func (in TaskSubmitInput) GetObtainTime() string          { return in.ObtainTime }
func (in TaskSubmitInput) GetSpecialtyTechnology() string { return in.SpecialtyTechnology }
func (in TaskSubmitInput) GetLikeSpecialty1() string      { return in.LikeSpecialty1 }
func (in TaskSubmitInput) GetLikeSpecialty2() string      { return in.LikeSpecialty2 }
func (in TaskSubmitInput) GetLikeSpecialty3() string      { return in.LikeSpecialty3 }

// TaskAddCirclePayload 是 SDK 内部使用的 addCircle 完整请求体。
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

// 兼容旧调用方
type TaskSubmitPayload = TaskAddCirclePayload

// TaskResult 是 addCircle 的业务返回摘要。
type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// TaskCircleTypeInfo 是 getCircleTypeByTaskId 返回的任务提交元数据。
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

// TaskEditInput 是修改写实记录的最小输入。
type TaskEditInput struct {
	ID                  int64
	TaskID              int64
	Content             string
	ImagePaths          []string
	ImageIDs            []int64
	PlayRole            string
	Address             string
	Level               string
	Name                string
	HostName            string
	CircleDate          string
	Rank                string
	ActivityName        string
	SportsName          string
	TeamName            string
	OrgName             string
	ResultsName         string
	ObtainTime          string
	SpecialtyTechnology string
	LikeSpecialty1      string
	LikeSpecialty2      string
	LikeSpecialty3      string
}

func (in TaskEditInput) Validate() error {
	if in.ID <= 0 {
		return ErrTaskInputIDRequired
	}
	if in.TaskID <= 0 {
		return ErrTaskInputTaskIDRequired
	}
	if strings.TrimSpace(in.Content) == "" {
		return ErrTaskInputContentRequired
	}
	return nil
}

// TaskInput 接口实现：TaskEditInput 的 ID 字段用于修改已有记录。
func (in TaskEditInput) GetID() *int64                  { return &in.ID }
func (in TaskEditInput) GetTaskID() int64               { return in.TaskID }
func (in TaskEditInput) GetContent() string             { return in.Content }
func (in TaskEditInput) GetImagePaths() []string        { return in.ImagePaths }
func (in TaskEditInput) GetImageIDs() []int64           { return in.ImageIDs }
func (in TaskEditInput) GetPlayRole() string            { return in.PlayRole }
func (in TaskEditInput) GetAddress() string             { return in.Address }
func (in TaskEditInput) GetLevel() string               { return in.Level }
func (in TaskEditInput) GetName() string                { return in.Name }
func (in TaskEditInput) GetHostName() string            { return in.HostName }
func (in TaskEditInput) GetCircleDate() string          { return in.CircleDate }
func (in TaskEditInput) GetRank() string                { return in.Rank }
func (in TaskEditInput) GetActivityName() string        { return in.ActivityName }
func (in TaskEditInput) GetSportsName() string          { return in.SportsName }
func (in TaskEditInput) GetTeamName() string            { return in.TeamName }
func (in TaskEditInput) GetOrgName() string             { return in.OrgName }
func (in TaskEditInput) GetResultsName() string         { return in.ResultsName }
func (in TaskEditInput) GetObtainTime() string          { return in.ObtainTime }
func (in TaskEditInput) GetSpecialtyTechnology() string { return in.SpecialtyTechnology }
func (in TaskEditInput) GetLikeSpecialty1() string      { return in.LikeSpecialty1 }
func (in TaskEditInput) GetLikeSpecialty2() string      { return in.LikeSpecialty2 }
func (in TaskEditInput) GetLikeSpecialty3() string      { return in.LikeSpecialty3 }

var (
	ErrTaskInputIDRequired      = taskInputError("id 为必填且必须 > 0")
	ErrTaskInputTaskIDRequired  = taskInputError("taskId 为必填且必须 > 0")
	ErrTaskInputContentRequired = taskInputError("content 为必填")
)

var submittedBlacklist = map[string]struct{}{
	"未提交":     {},
	"上传期 未提交": {},
}

func (t *Task) SetSubmittedByStatus() {
	_, ok := submittedBlacklist[t.CircleTaskStatus]
	t.Submitted = !ok
}

type taskInputError string

func (e taskInputError) Error() string { return string(e) }
