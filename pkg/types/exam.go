package types

import "time"

// ExamResult 学生成绩记录（来自 queryStudentExam 接口）。
type ExamResult struct {
	ID          int64     `json:"id"`          // 成绩记录 ID
	StudentID   int64     `json:"studentId"`   // 学生 ID
	StudentName string    `json:"studentName"` // 学生姓名
	ClassName   string    `json:"className"`   // 班级名称
	GradeName   string    `json:"gradeName"`   // 年级名称
	CourseName  string    `json:"courseName"`  // 课程名称
	ExamName    string    `json:"examName"`    // 考试名称
	Score       float64   `json:"score"`       // 分数
	TermID      int64     `json:"termId"`      // 学期 ID
	TermName    string    `json:"termName"`    // 学期名称
	CreateTime  time.Time `json:"createTime"`  // 创建时间
}

// TermInfo 学期信息（来自 getInitInfo / pageQueryTermBySchoolId 接口）。
type TermInfo struct {
	ID       int64  `json:"id"`       // 学期 ID
	Name     string `json:"name"`     // 学期名称
	Flag     int    `json:"flag"`     // 当前学期标志（1=当前学期）
	StartDate string `json:"startDate"` // 开始日期
	EndDate  string `json:"endDate"`   // 结束日期
}

// ExamInitInfo 成绩初始化信息（来自 getInitInfo 接口）。
type ExamInitInfo struct {
	TermList   []TermInfo `json:"termList"`   // 学期列表
	ExamList   []ExamType `json:"examList"`   // 考试列表
	CourseList []Course   `json:"courseList"` // 课程列表
}

// ExamType 考试类型。
type ExamType struct {
	ID   int64  `json:"id"`   // 考试 ID
	Name string `json:"name"` // 考试名称
}

// Course 课程信息。
type Course struct {
	CourseID   int64  `json:"course_id"`   // 课程 ID
	CourseName string `json:"courseName"`  // 课程名称
}
