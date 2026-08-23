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

// 写实等级常量（对应服务端 level，对齐前端管理端字典与展示映射）。
//
// 前端来源（原生 src 对照）：
//   - 获取字典：GET /api/common/sys/dict/list?cateCode=23 → 填充下拉（level 列表）
//   - 展示映射：managementRightBottom.vue / yhmanagement 同步的 switch(map.level)
//     1=国家  2=省  3=地区/市  4=区/县/街道/社区  5=校  6=年段
//   - 提交校验：checkData 中 rank/level 成对校验（部分 targetName）
//
// SDK 约定：空串原样发送，不发明 "5"；展示名与字典名以服务端为准，此处常量仅为调用方显式赋值时的可读别名。
const (
	TaskLevelNational    = "1" // 国家
	TaskLevelProvince    = "2" // 省
	TaskLevelCity        = "3" // 地区/市
	TaskLevelCounty      = "4" // 区/县/街道/社区
	TaskLevelSchool      = "5" // 校
	TaskLevelGrade       = "6" // 年段
)

// TaskLevelName 返回等级代码对应的展示名（1..6），未知返回空串。
func TaskLevelName(code string) string {
	switch code {
	case TaskLevelNational:
		return "国家"
	case TaskLevelProvince:
		return "省"
	case TaskLevelCity:
		return "地区/市"
	case TaskLevelCounty:
		return "区/县/街道/社区"
	case TaskLevelSchool:
		return "校"
	case TaskLevelGrade:
		return "年段"
	default:
		return ""
	}
}

// 审核情况常量（对应服务端 checkResult，前端写实表单）。
//
// 前端来源（原生 src）：
//   - managementRightTop/Bottom 的 switch(map.check_result): 1=优秀 2=良 3=合格 4=差
//   - 表单中部分 targetName 用 radio（1/3），部分用 select；展示映射一致。
const (
	CheckResultExcellent = "1" // 优秀
	CheckResultGood       = "2" // 良
	CheckResultPass       = "3" // 合格
	CheckResultPoor       = "4" // 差
)

// CheckResultName 返回审核情况代码的展示名。
func CheckResultName(code string) string {
	switch code {
	case CheckResultExcellent:
		return "优秀"
	case CheckResultGood:
		return "良"
	case CheckResultPass:
		return "合格"
	case CheckResultPoor:
		return "差"
	default:
		return ""
	}
}

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
	GetTermName() string
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
	// 新增（v1.4.0）
	GetHours() string
	GetCircleBeginDate() string
	GetCircleEndDate() string
	GetCheckResult() string
	GetPatentType() string
	GetPatentNum() string
}

// TaskSubmitInput 是公开给 SDK 调用方的最小任务提交输入。
//
// 用户应填：TaskID（选任务）、Content、按任务类型的活动字段、图片；
// Address/OrgName/Level/PlayRole 等按活动类型由调用方填写（前端手填，空串原样提交）；
// Hours：任务 getCircleTypeByTaskId 的 hours>0 时可省略（SDK 用元数据，对齐前端只读自动填）；
// 任务 hours≤0 时必须显式填写（对齐前端可编辑 + checkData）。
// SDK 自动：circleTaskId/circleTypeId/dimensionId（元数据）、pictureList（上传）。
// 不再发明：空 Address/OrgName 不填学校名、空 Level 不默认 "5"（与前端一致）。
// CircleDate/TermName 前端无 v-model，非用户输入，仅兼容保留。
//
// Go 直调提醒：本结构体全部活动字段均为 string（含 Hours/Level/PlayRole/CheckResult 等）；
// Go 调用方如源数据为 number，须自行转为 string 后再赋值（例如 fmt.Sprintf("%v", v)），
// number→string 的兼容仅在 CLI --payload 边界生效（cmd/nazhi 私有 JSON helper），SDK 侧不做自动类型转换。
//
// 校验策略（有意设计）：Validate 仅校验 TaskID>0 && Content 非空，不复制前端 14 分支条件必填；
// 调用方需按活动类型（targetName 1-14）自行保证必填字段，与前端 checkData 对齐；
// 服务端仍会做最终业务校验，缺字段将以 ErrBusinessRejected 返回。
type TaskSubmitInput struct {
	TaskID     int64
	Content    string
	ImagePaths []string
	ImageIDs   []int64
	PlayRole   string
	Address    string
	Level      string

	Name     string
	HostName string
	// CircleDate / TermName：前端 form 有键但**无 v-model**，用户从不手填。
	// 仅兼容旧调用方；推荐保持空串，勿当作用户必填字段。
	CircleDate          string
	TermName            string
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
	// Hours：可选字符串。空且任务预设>0 → 用元数据；空且预设≤0 → 校验失败；非空优先用户值。
	Hours           string
	CircleBeginDate string
	CircleEndDate   string
	CheckResult     string
	PatentType      string
	PatentNum       string
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
func (in TaskSubmitInput) GetTermName() string            { return in.TermName }
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
func (in TaskSubmitInput) GetHours() string               { return in.Hours }
func (in TaskSubmitInput) GetCircleBeginDate() string     { return in.CircleBeginDate }
func (in TaskSubmitInput) GetCircleEndDate() string       { return in.CircleEndDate }
func (in TaskSubmitInput) GetCheckResult() string         { return in.CheckResult }
func (in TaskSubmitInput) GetPatentType() string          { return in.PatentType }
func (in TaskSubmitInput) GetPatentNum() string           { return in.PatentNum }

// TaskAddCirclePayload 是 SDK 内部使用的 addCircle 完整请求体。
type TaskAddCirclePayload struct {
	ID                  *int64  `json:"id,omitempty"`
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
	TermName            string
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
	// 新增（v1.4.0）
	Hours           string
	CircleBeginDate string
	CircleEndDate   string
	CheckResult     string
	PatentType      string
	PatentNum       string
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
func (in TaskEditInput) GetTermName() string            { return in.TermName }
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
func (in TaskEditInput) GetHours() string               { return in.Hours }
func (in TaskEditInput) GetCircleBeginDate() string     { return in.CircleBeginDate }
func (in TaskEditInput) GetCircleEndDate() string       { return in.CircleEndDate }
func (in TaskEditInput) GetCheckResult() string         { return in.CheckResult }
func (in TaskEditInput) GetPatentType() string          { return in.PatentType }
func (in TaskEditInput) GetPatentNum() string           { return in.PatentNum }

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

// SetNeedPicFromUpPic 用平台 upPic（int 0/1）推导 NeedPic。
//
// getCircleStatistics 只返回 upPic，不返回 needPic；encoding/json 不会把
// upPic 填进 NeedPic。调用方依赖 NeedPic 做"是否要求图片"时，必须在解码后调用本方法。
func (t *Task) SetNeedPicFromUpPic() {
	if t == nil {
		return
	}
	t.NeedPic = t.UpPic > 0
}

type taskInputError string

func (e taskInputError) Error() string { return string(e) }
