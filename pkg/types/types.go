// Package types 定义 nazhi-cli SDK 的全部公共类型。
package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// ─── 认证 ───

// LoginRequest 是目标平台 SSO 登录请求。
type LoginRequest struct {
	SchoolID string // 学校 ID（可为空，服务端自学号推断）
	Username string // 学号
	Password string // 密码
}

// LoginResponse 是 SSO 登录成功后的响应。
//
// 字段约定：本结构只包含登录 token + expires 信息，
// **不再** 提供用户基本信息字段。用户基本信息请通过 Client.GetMyInfo()
// 单独获取（GetMyInfo 返回 *UserInfo，含完整 51 字段）。
//
// 实际被填充的字段仅为 Token / ExpiresAt / RawData / FallbackUsed 四件套。
type LoginResponse struct {
	Token        string         `json:"token"`         // X-Auth-Token
	ExpiresAt    time.Time      `json:"expires_at"`    // 过期时间
	RawData      map[string]any `json:"-"`             // 登录响应完整原始数据
	FallbackUsed bool           `json:"fallback_used"` // 本次登录是否使用了 fallback OCR（primary 失败后降级到 ddddocr）
}

// ─── 用户 ───

// UserInfo 是用户个人资料。
//
// 字段命名策略：
//   - 平台返回的所有字段都已暴露（30+ 字段），便于脚本直接访问
//   - 生日使用 birthdayStr（字符串版），避开 birthday 数组的类型不匹配问题
//   - 时间戳同时暴露数组（如 creationTime [y,m,d,h,m,s]）和字符串（*TimeStr）两种形式
//   - nullable 字段（null）解析为零值（int=0, string=""），调用方用 if 判断即可
type UserInfo struct {
	// 基础身份
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	Initials              string `json:"initials"`              // 姓名首字母（如 "zs"）
	Pinyin                string `json:"pinyin"`                // 姓名全拼
	StudentNumber         string `json:"studentNumber"`         // 学号
	StudentID             int64  `json:"studentId"`             // 学生 ID
	StudyNumber           string `json:"studyNumber"`           // 校内短学号
	NationalStudentNumber string `json:"nationalStudentNumber"` // 全国学号

	// 学校 / 班级 / 年级
	//
	// SchoolID 学校 ID。
	// JSON tag 为 schoolId（驼峰）——getMyInfo API 使用此格式。
	// 注意：
	//   - getSchoolIdByStudentNumber 使用 school_id（蛇形）作为 JSON key，
	//     由 GetSchoolID 以 string 形式返回，最终由调用方（Login 等）使用
	//   - SelfEvalStatus.SchoolID 使用 school_id（蛇形）作为 JSON tag
	// 两条 API 路径（getMyInfo vs getSchoolIdByStudentNumber）的 JSON 命名风格
	// 不同，但数据类型最终都会归一为 int64 或 string。
	SchoolID   int64  `json:"schoolId"`
	SchoolName string `json:"schoolName"` // 平台返回 null 时为空字符串
	GradeID    int64  `json:"gradeId"`
	GradeName  string `json:"gradeName"`
	ClassID    int64  `json:"classId"`
	ClassName  string `json:"className"`
	Level      int    `json:"level"` // 年级代码

	// 座号
	Seat     int `json:"seat"`
	SeatSort int `json:"seatSort"`

	// 性别
	Gender     int    `json:"gender"`
	GenderName string `json:"genderName"`

	// 民族 / 证件
	Nation int    `json:"nation"` // 民族代码（1=汉族）
	IDType int    `json:"idType"` // 证件类型
	IDCard string `json:"idCard"`

	// 生日
	//   - Birthday (string) 兼容 server 返回的 birthdayStr 字符串
	//   - BirthdayDate (struct) 兼容 server 返回的 [y,m,d] 数组（双形态容错）
	// 业务方按需使用；缺失字段为类型零值
	Birthday     string        `json:"birthdayStr"`
	BirthdayDate *BirthdayDate `json:"birthday,omitempty"`

	// 联系方式
	Telephone      string `json:"telephone"`      // 电话
	Email          string `json:"email"`          // 邮箱
	CurrentAddress string `json:"currentAddress"` // 现地址
	ContactAddress string `json:"contactAddress"` // 联系地址
	FamilyAddress  string `json:"familyAddress"`  // 家庭地址
	NativePlace    string `json:"nativePlace"`    // 籍贯

	// 学籍状态
	Status             int    `json:"status"`             // 学籍状态码
	StatusName         string `json:"statusName"`         // 学籍状态名（如 "在籍"）
	PositionID         int    `json:"positionId"`         // 职位 ID
	PositionName       string `json:"positionName"`       // 职位名（常为 null）
	YouthLeagueFlag    int    `json:"youthLeagueFlag"`    // 团员标志（1=团员）
	CriminalRecordFlag int    `json:"criminalRecordFlag"` // 犯罪记录标志

	// 爱好
	Hobbies string `json:"hobbies"`

	// 入学时间（数组 + 字符串两种形式）
	AdmissionDate    []int  `json:"admissionDate"`    // [2025,9,1]
	AdmissionDateStr string `json:"admissionDateStr"` // 常为 null

	// 创建时间（数组 + 字符串）
	CreationTime    []int  `json:"creationTime"`    // [2025,10,9,10,32,6]
	CreationTimeStr string `json:"creationTimeStr"` // "2025-10-09 10:32:06"

	// 修改时间（数组 + 字符串）
	ModifyTime    []int  `json:"modifyTime"`    // [2026,2,6,10,16,15]
	ModifyTimeStr string `json:"modifyTimeStr"` // "2026-02-06 10:16:15"

	// 创建/修改人
	Creator  int `json:"creator"`
	Modifier int `json:"modifier"`

	// 照片附件
	PhotoAttachmentID int64 `json:"photoAttachmentId"` // 平台返回 null → 0

	// 积分
	TotalPoints int `json:"totalPoints"`
	UsedPoints  int `json:"usedPoints"`

	// 其他
	StudentUUID string `json:"studentUuid"` // 平台返回 null → ""
}

// ─── 任务 ───

// BirthdayDate 生日结构体（兼容 [2009,12,11] 数组和 "2009-12-11" 字符串）。
// 解决 ef5c1ad 移除 ac9e084 自定义解析后丢失的"双形态容错"能力——
// 若 server 升级只返回 birthday 数组（无 birthdayStr），BirthdayDate 仍可解析。
type BirthdayDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

// String 返回 "YYYY-MM-DD" 格式。
func (b BirthdayDate) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", b.Year, b.Month, b.Day)
}

// UnmarshalJSON 接受三种 JSON 形式：
//   - null            → 零值
//   - "2009-12-11"    → 解析字符串
//   - [2009,12,11]    → 解析数组
func (b *BirthdayDate) UnmarshalJSON(data []byte) error {
	// null
	if string(data) == "null" {
		return nil
	}
	// 字符串
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			t, err = time.Parse(time.RFC3339, s)
			if err != nil {
				return fmt.Errorf("birthday 字符串格式错误: %w", err)
			}
		}
		b.Year, b.Month, b.Day = t.Year(), int(t.Month()), t.Day()
		return nil
	}
	// 数组
	var arr []int
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) < 3 {
			return fmt.Errorf("birthday 数组长度不足 3: %d", len(arr))
		}
		b.Year, b.Month, b.Day = arr[0], arr[1], arr[2]
		return nil
	}
	return fmt.Errorf("birthday 格式无法识别: %s", string(data))
}

// Task 是目标平台的一个任务条目。
type Task struct {
	ID            int64   `json:"id"`               // 任务 ID（即 circleTaskId）
	Name          string  `json:"name"`             // 任务名称
	CircleTypeID  int64   `json:"circleTypeId"`     // 圈子类型 ID
	TypeName      string  `json:"typeName"`         // 类型名称
	DimensionID   int64   `json:"dimensionId"`      // 维度 ID
	DimensionName string  `json:"dimensionName"`    // 维度名称
	Hours         float64 `json:"hours"`            // 学时
	Status        string  `json:"circleTaskStatus"` // 任务状态
	PushNum       int     `json:"pushNum"`          // 推送次数
	UpPic         int     `json:"upPic"`            // 1=需要图片
	Score         float64 `json:"score"`
	StartDate     string  `json:"startDateStr"` // 平台返回 "2026-01-12"
	EndDate       string  `json:"endDateStr"`   // 平台返回 "2026-02-10"
	CreatorName   string  `json:"creatorName"`
	RoleName      string  `json:"roleName"`
	TermID        int64   `json:"termId"`
	ScopeType     int     `json:"scopeType"`
	ScopeTypeName string  `json:"scopeTypeName"`
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

// ─── 写实记录 ───

// PageBean 目标平台通用分页信息。
type PageBean struct {
	PageNo    int `json:"pageNo"`
	PageSize  int `json:"pageSize"`
	TotalNum  int `json:"totalNum"`
	TotalPage int `json:"totalPage"`
}

// CircleRecord 一条已提交的写实记录（来自 getStudentCircle 接口）。
type CircleRecord struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	Content      string        `json:"content"`
	CircleTaskID int64         `json:"circle_task_id"`
	CircleTypeID int64         `json:"circle_type_id"`
	DimensionID  int64         `json:"dimension_id"`
	TypeName     string        `json:"type_name"`
	Status       int           `json:"status"` // 0=待审, 1=已通过
	CircleDate   string        `json:"circle_date"`
	Hours        float64       `json:"hours"`
	StudentID    int64         `json:"studentId"`
	ClassName    string        `json:"class_name"`
	IfMySelf     int           `json:"ifMySelf"`
	ImgList      []CircleImage `json:"imgList"`
	Remark       string        `json:"remark"`
}

// CircleImage 写实记录关联的图片附件。
type CircleImage struct {
	ID           int64 `json:"id"`
	CircleID     int64 `json:"circle_id"`
	AttachmentID int64 `json:"attachment_id"`
	TaskID       int64 `json:"task_id"`
	ClassID      int64 `json:"class_id"`
}

type TaskResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
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

// ─── 荣誉 ───

// HonorType 一种可申报的荣誉类型（来自 getHonorType 接口）。
type HonorType struct {
	ID            int64  `json:"id"`             // 荣誉类型 ID
	Name          string `json:"name"`           // 荣誉名称
	LevelName     string `json:"level_name"`     // 级别名（校/区县/市/省/国家）
	Level         int    `json:"level"`          // 级别代码（5=校, 4=区县, 3=市, 2=省, 1=国家）
	Score         string `json:"score"`          // 分数说明（如"分数：+5.0"）
	DimensionID   int64  `json:"dimension_id"`   // 所属维度 ID
	DimensionName string `json:"dimension_name"` // 维度名
	SortNo        int    `json:"sort_no"`        // 排序号
}

// HonorRecord 一条已申报的荣誉记录（来自 getHonorByStudentId 接口）。
type HonorRecord struct {
	ID               int64   `json:"id"`                // 荣誉记录 ID
	TypeName         string  `json:"type_name"`         // 荣誉类型名
	TypeID           int64   `json:"type_id"`           // 荣誉类型 ID
	Level            int     `json:"level"`             // 级别代码
	LevelName        string  `json:"level_name"`        // 级别名
	DimensionID      int64   `json:"dimension_id"`      // 维度 ID
	DimensionName    string  `json:"dimension_name"`    // 维度名
	Score            float64 `json:"score"`             // 分数
	Status           int     `json:"status"`            // 0=未审核, 1=审核通过
	StatusName       string  `json:"statusName"`        // 状态名
	StudentName      string  `json:"student_name"`      // 学生姓名
	ClassName        string  `json:"class_name"`        // 班级名
	GetDate          string  `json:"get_date"`          // 获得日期
	EvaluationAgency string  `json:"evaluation_agency"` // 颁发机构
	IfShow           string  `json:"ifshow"`            // 是否展示
	AuditorName      string  `json:"auditor_name"`      // 审核人
	ShowReportFlag   int     `json:"show_report_flag"`  // 是否在报告中展示
}

// AddHonorPayload 是 addHonor 接口的请求体。
type AddHonorPayload struct {
	Name                string `json:"name"`                // 荣誉名称
	TypeID              int64  `json:"typeId"`              // 荣誉类型 ID
	TypeName            string `json:"typeName"`            // 荣誉类型名
	Level               int    `json:"level"`               // 级别代码
	EvaluationAgency    string `json:"evaluationAgency"`    // 颁发机构
	GetDate             string `json:"getDate"`             // 获得日期（YYYY-MM-DD）
	CertImgAttachmentID string `json:"certImgAttachmentId"` // 证书图片附件 ID 或空
}

// HonorSelectOption 是下拉选择选项（label/value 对）。
type HonorSelectOption struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}
