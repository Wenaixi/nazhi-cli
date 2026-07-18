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
// v1.1.2 fix（submitted 判定）：
//
//	服务端返回 circleTaskStatus 字符串，SDK 解码后根据规则映射为 submitted bool：
//	  "未提交" 或 "上传期 未提交" → submitted = false
//	  其他任意值（"已提交"/"审核中"/"已通过"/空串等）→ submitted = true
//
// v1.2.0 扩展：新增服务端 getCircleStatistics 返回的全部原始字段（共 40 字段），
// 补齐之前 ~18 个缺失字段。新字段均使用 omitempty，零值/null 时不在输出中出现，
// 完美兼容已有输出版本。
type Task struct {
	ID               int64    `json:"id"`                // 任务 ID（即 circleTaskId）
	Name             string   `json:"name"`              // 任务名称
	TypeName         string   `json:"typeName"`          // 类型名称
	DimensionName    string   `json:"dimensionName"`     // 维度名称
	Hours            float64  `json:"hours"`             // 学时
	Score            float64  `json:"score"`             // 学分（与 hours 不同）
	Remark           string   `json:"remark"`            // 任务说明（"照片加描述" 等）
	CircleTaskStatus string   `json:"circleTaskStatus"`  // 服务端原始状态字符串
	Submitted        bool     `json:"submitted"`         // 是否已提交（根据 circleTaskStatus 映射，见 SetSubmittedByStatus）
	NeedPic          bool     `json:"needPic"`           // 是否需要图片（来自服务端 upPic 0/1）
	StartDate        DateOnly `json:"startDateStr"`      // 开始日期（来自服务端 startDateStr，如 2026-01-12）
	EndDate          DateOnly `json:"endDateStr"`        // 结束日期（来自服务端 endDateStr，如 2026-02-10）
	AuditStartDate   DateOnly `json:"auditStartDateStr"` // 审核开始日期
	AuditEndDate     DateOnly `json:"auditEndDateStr"`   // 审核截止日期
	CreatorName      string   `json:"creatorName"`       // 创建者（"许风华"/"管理员"）
	RoleName         string   `json:"roleName"`          // 创建者角色（"班主任"）
	CreationTime     []int    `json:"creationTime"`      // 任务创建时间（[y,m,d,h,m,s] 数组，Java LocalDateTime）
	CreationTimeStr  DateOnly `json:"creationTimeStr"`   // 任务创建日期字符串（YYYY-MM-DD）
	TermID           int64    `json:"termId"`            // 学期 ID
	PushNum          int      `json:"pushNum"`           // 推送次数
	ScopeType        int      `json:"scopeType"`         // 作用域类型（参见 ScopeClass/ScopeGrade/ScopeStage）
	ScopeTypeName    string   `json:"scopeTypeName"`     // 作用域名称

	// v1.2.0 新增：服务端 getCircleStatistics 返回的全部原始字段
	// 所有新字段使用 omitempty，零值/null 时不在 JSON 输出中出现。
	SchoolID           int64    `json:"schoolId,omitempty"`           // 学校 ID
	CircleTypeID       int64    `json:"circleTypeId,omitempty"`       // 写实类型 ID
	Creator            int64    `json:"creator,omitempty"`            // 创建者用户 ID
	Modifier           *int64   `json:"modifier,omitempty"`           // 最后修改者 ID（可为 null）
	ModifyTime         []int    `json:"modifyTime,omitempty"`         // 最后修改时间 [y,m,d,h,m,s]（可为 null）
	RoleID             int64    `json:"roleId,omitempty"`             // 角色 ID
	AuditorSubjectID   *int64   `json:"auditorSubjectId,omitempty"`   // 审核学科 ID（可为 null）
	StateType          int      `json:"stateType,omitempty"`          // 状态类型（3=正常）
	AreaID             int64    `json:"areaId,omitempty"`             // 区域 ID
	AreaTaskID         int64    `json:"areaTaskId,omitempty"`         // 区域任务 ID
	UpPic              int      `json:"upPic,omitempty"`              // 需上传图片原始值 0/1（needPic 的 int 源）
	EvaluatedNumber    *int     `json:"evaluatedNumber,omitempty"`    // 已评价人数（可为 null）
	UnEvaluatedNumber  *int     `json:"unEvaluatedNumber,omitempty"`  // 未评价人数（可为 null）
	UnsubmittedNumber  *int     `json:"unsubmittedNumber,omitempty"`  // 未提交人数（可为 null）
	SubmitNumber       int      `json:"submitNumber,omitempty"`       // 提交人数
	PictureList        []int64  `json:"pictureList,omitempty"`        // 图片附件 ID 列表（可为 null）
	ClassID            *int64   `json:"classId,omitempty"`            // 班级 ID（可为 null，学段任务无）
	GradeID            *int64   `json:"gradeId,omitempty"`            // 年级 ID（可为 null）
}

// TaskSubmitInput 是公开给 SDK 调用方的最小任务提交输入。
//
// 设计目标：调用方只提供真正需要人工决策的字段；其余 30 字段 payload 由 SDK
// 内部根据 taskId 元数据、用户资料和上传结果自动组装。
//
// v1.2.0 新增 14 个可选字段，暴露前端 addCircle 请求体全部参数。
// 所有新增字段为零值空串时维持原有 fallback 行为，不影响现有调用方。
type TaskSubmitInput struct {
	TaskID     int64    // 必填：任务 ID
	Content    string   // 必填：心得/感悟
	ImagePaths []string // 可选：本地图片路径列表
	ImageIDs   []int64  // 可选：已上传的附件 ID 列表，避免重复上传
	PlayRole   string   // 可选：默认空串，显式传入时覆盖
	Address    string   // 可选：为空时默认 schoolName，允许调用方覆盖
	Level      string   // 可选：为空时默认校级（5），允许调用方覆盖

	// v1.2.0 新增：addCircle 请求体全部可选参数
	// 零值空串时 buildTaskSubmitPayload 保持原行为（HAR 确认前端传空串）
	Name                string // addCircle.name — 任务/活动名称（通常与任务名相同，允许自定义）
	HostName            string // addCircle.hostName — 主持人
	CircleDate          string // addCircle.circleDate — 写实日期
	Rank                string // addCircle.rank — 排名/等第
	ActivityName        string // addCircle.activityName — 活动名称
	SportsName          string // addCircle.sportsName — 体育项目名称
	TeamName            string // addCircle.teamName — 团队名称
	OrgName             string // addCircle.orgName — 组织单位名称（为空时默认学校名）
	ResultsName         string // addCircle.resultsName — 成果名称
	ObtainTime          string // addCircle.obtainTime — 获得时间
	SpecialtyTechnology string // addCircle.specialtyTechnology — 特长/技术
	LikeSpecialty1      string // addCircle.likeSpecialty1 — 爱好特长 1
	LikeSpecialty2      string // addCircle.likeSpecialty2 — 爱好特长 2
	LikeSpecialty3      string // addCircle.likeSpecialty3 — 爱好特长 3
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

// TaskEditInput 是修改写实记录的最小输入。
//
// 与 TaskSubmitInput 字段完全对齐，新增 id 字段（必填，写实记录主键）。
// 其余字段语义与 TaskSubmitInput 一致。
type TaskEditInput struct {
	ID                  int64    // 必填：写实记录 ID（来自 getStudentCircle 列表中的 id 字段）
	TaskID              int64    // 必填：任务 ID（circle_task_id）
	Content             string   // 必填：心得/感悟
	ImagePaths          []string // 可选：本地图片路径列表
	ImageIDs            []int64  // 可选：已上传的附件 ID 列表，避免重复上传
	PlayRole            string   // 可选：承担角色
	Address             string   // 可选：地点
	Level               string   // 可选：级别
	Name                string   // 可选：任务/活动名称
	HostName            string   // 可选：主持人
	CircleDate          string   // 可选：写实日期
	Rank                string   // 可选：排名/等第
	ActivityName        string   // 可选：活动名称
	SportsName          string   // 可选：体育项目名称
	TeamName            string   // 可选：团队名称
	OrgName             string   // 可选：组织单位名称
	ResultsName         string   // 可选：成果名称
	ObtainTime          string   // 可选：获得时间
	SpecialtyTechnology string   // 可选：特长/技术
	LikeSpecialty1      string   // 可选：爱好特长 1
	LikeSpecialty2      string   // 可选：爱好特长 2
	LikeSpecialty3      string   // 可选：爱好特长 3
}

// Validate 校验修改写实记录的输入。
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

var (
	ErrTaskInputIDRequired      = taskInputError("id 为必填且必须 > 0")
	ErrTaskInputTaskIDRequired  = taskInputError("taskId 为必填且必须 > 0")
	ErrTaskInputContentRequired = taskInputError("content 为必填")
)

// submittedBlacklist 是 submitted=false 的 circleTaskStatus 值集合。
// v1.1.2 fix：只有明确标记"未提交"（含带前缀的变体）才判定为未完成，
// 其他任意值（"已提交"/"审核中"/"已通过"/空串等）均视为已完成。
var submittedBlacklist = map[string]struct{}{
	"未提交":     {},
	"上传期 未提交": {},
}

// SetSubmittedByStatus 根据 circleTaskStatus 设置 submitted 字段。
// 规则：仅当状态在黑名单中时为 false，其余均为 true。
func (t *Task) SetSubmittedByStatus() {
	_, ok := submittedBlacklist[t.CircleTaskStatus]
	t.Submitted = !ok
}

type taskInputError string

func (e taskInputError) Error() string { return string(e) }
