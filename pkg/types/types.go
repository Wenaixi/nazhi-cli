// Package types 定义 nazhi-cli SDK 的全部公共类型。
package types

import "time"

// ─── 认证 ───

// LoginRequest 是目标平台 SSO 登录请求。
type LoginRequest struct {
	SchoolID string // 学校 ID（可为空，服务端自学号推断）
	Username string // 学号
	Password string // 密码
}

// LoginResponse 是 SSO 登录成功后的响应。
type LoginResponse struct {
	Token        string         `json:"token"`         // X-Auth-Token
	RefreshAfter time.Time      `json:"refresh_after"` // 推荐刷新时间
	ExpiresAt    time.Time      `json:"expires_at"`    // 过期时间
	UserInfo     *UserInfo      `json:"user_info"`     // 用户基本信息
	RawData      map[string]any `json:"-"`             // 登录响应完整原始数据
}

// ─── 用户 ───

type UserInfo struct {
	ID            int64          `json:"id"`
	Name          string         `json:"name"`
	StudentNumber string         `json:"studentNumber"`
	StudentID     int64          `json:"studentId"`
	SchoolID      int64          `json:"schoolId"`
	SchoolName    string         `json:"schoolName"`
	GradeName     string         `json:"gradeName"`
	ClassName     string         `json:"className"`
	Seat          int            `json:"seat"`
	Gender        int            `json:"gender"`
	GenderName    string         `json:"genderName"`
	IDCard        string         `json:"idCard"`
	Birthday      string         `json:"birthday"`
	StudyNumber   string         `json:"studyNumber"`
	Raw           map[string]any `json:"-"` // 完整原始数据
}

// ─── 学校 ───

// SchoolInfo 是目标平台的学校基本信息。
type SchoolInfo struct {
	SchoolID   string `json:"school_id"`
	SchoolName string `json:"school_name"`
}

// ─── 任务 ───

// Task 是目标平台的一个任务条目。
type Task struct {
	ID            int64   `json:"id"`            // 任务 ID（即 circleTaskId）
	Name          string  `json:"name"`           // 任务名称
	CircleTypeID  int64   `json:"circleTypeId"`   // 圈子类型 ID
	TypeName      string  `json:"typeName"`       // 类型名称
	DimensionID   int64   `json:"dimensionId"`    // 维度 ID
	DimensionName string  `json:"dimensionName"`  // 维度名称
	Hours         float64 `json:"hours"`          // 学时
	Status        string  `json:"circleTaskStatus"` // 任务状态
	PushNum       int     `json:"pushNum"`        // 推送次数
	UpPic         int     `json:"upPic"`          // 1=需要图片
	Score         float64 `json:"score"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	CreatorName   string  `json:"creatorName"`
	RoleName      string  `json:"roleName"`
	TermID        int64   `json:"termId"`
	ScopeType     int     `json:"scopeType"`
	ScopeTypeName string  `json:"scopeTypeName"`
}

// TaskSubmitPayload 是 addCircle 接口的完整请求体（29 字段透传）。
type TaskSubmitPayload struct {
	ID                 *int64   `json:"id"`
	Name               string   `json:"name"`
	HostName           string   `json:"hostName"`
	CircleDate         string   `json:"circleDate"`
	Rank               string   `json:"rank"`
	Level              string   `json:"level"`
	Content            string   `json:"content"`
	PictureList        []int64  `json:"pictureList"`
	CircleTaskID       int64    `json:"circleTaskId"`
	CircleTypeID       int64    `json:"circleTypeId"`
	DimensionID        int64    `json:"dimensionId"`
	Hours              float64  `json:"hours"`
	CircleBeginDate    string   `json:"circleBeginDate"`
	CircleEndDate      string   `json:"circleEndDate"`
	CheckResult        string   `json:"checkResult"`
	PatentType         string   `json:"patentType"`
	PatentNum          string   `json:"patentNum"`
	Address            string   `json:"address"`
	TermName           string   `json:"termName"`
	ActivityName       string   `json:"activityName"`
	SportsName         string   `json:"sportsName"`
	TeamName           string   `json:"teamName"`
	OrgName            string   `json:"orgName"`
	ResultsName        string   `json:"resultsName"`
	ObtainTime         string   `json:"obtainTime"`
	SpecialtyTechnology string  `json:"specialtyTechnology"`
	PlayRole           string   `json:"playRole"`
	LikeSpecialty1     string   `json:"likeSpecialty1"`
	LikeSpecialty2     string   `json:"likeSpecialty2"`
	LikeSpecialty3     string   `json:"likeSpecialty3"`
}

// ─── 任务提交结果 ───

type TaskResult struct {
	Code int               `json:"code"`
	Msg  string            `json:"msg"`
	Raw  map[string]any    `json:"-"`
}

// ─── 维度 ───

type Dimension struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ─── 自我评价 ───

type SelfEvalStatus struct {
	StudentComment string `json:"student_comment"`
	TeacherComment string `json:"teacher_comment"`
	StudentName    string `json:"student_name"`
	StudentNumber  string `json:"student_number"`
	StudentID      int64  `json:"student_id"`
	ClassName      string `json:"class_name"`
	GradeName      string `json:"grade_name"`
	SchoolID       int64  `json:"school_id"`
	IsGrad         string `json:"is_grad"`
	EvalRecordID   int64  `json:"id"`
}

// ─── 会话激活结果 ───

// SessionInfo 是 ActivateSession 的返回信息。
type SessionInfo struct {
	UserInfo *UserInfo `json:"user_info"`
	Raw      map[string]any `json:"-"`
}
