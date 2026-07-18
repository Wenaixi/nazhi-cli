package types

// DemocraticActivity 民主评价活动（来自 getActivity 接口）。
type DemocraticActivity struct {
	ID          int64     `json:"id"`          // 活动 ID
	Name        string    `json:"name"`        // 活动名称
	SubPlanID   int64     `json:"subPlanId"`   // 子计划 ID
	EndDate     string    `json:"endDate"`     // 结束日期
	StartDate   string    `json:"startDate"`   // 开始日期
	Status      int       `json:"status"`      // 活动状态
	CreateTime  string `json:"createTime"`  // 创建时间（原始字符串）
}

// SelfEvaluationItem 自评项目（来自 getSelfEvaluation 接口）。
type SelfEvaluationItem struct {
	ID                int64  `json:"id"`                // 项目 ID
	ContentDetail     string `json:"contentDetail"`     // 评价内容
	EvaluationResult  string `json:"evaluationResult"`  // 评价结果
	EvaluationResult2 string `json:"evaluationResult2"` // 评价结果2
}

// MutualEvaluation 互评记录（来自 getMutualEvaluationDetail 接口）。
type MutualEvaluation struct {
	ID                     int64  `json:"id"`                     // 记录 ID
	StudentID              int64  `json:"studentId"`              // 学生 ID
	StudentName            string `json:"studentName"`            // 学生姓名
	StudentNumber          string `json:"studentNumber"`          // 学号
	MutualEvaluationResult string `json:"mutualEvaluationResult"` // 互评结果
	EvaluationStatus       int    `json:"evaluationStatus"`       // 评价状态
}

// DemocraticResult 民主评价结果（来自 getDemocraticResult 接口）。
type DemocraticResult struct {
	SelfEvaluation    string `json:"selfEvaluation"`    // 自评成绩
	MutualEvaluation  string `json:"mutualEvaluation"`  // 互评成绩
	TeacherEvaluation string `json:"teacherEvaluation"` // 教师评价成绩
}

// MutualPersonInfo 互评人员信息（来自 getMutualPersonInfo 接口）。
type MutualPersonInfo struct {
	EvaluatedNumbers    int                   `json:"evaluatedNumbers"`    // 已评价人数
	NotEvaluatedNumbers int                   `json:"notEvaluatedNumbers"` // 未评价人数
	ClassStudentList    []ClassStudent        `json:"classStudentList"`    // 班级学生列表
}

// ClassStudent 班级学生信息。
type ClassStudent struct {
	ID                     int64  `json:"id"`                     // 学生 ID
	Name                   string `json:"name"`                   // 学生姓名
	StudentNumber          string `json:"studentNumber"`          // 学号
	MutualEvaluationResult string `json:"mutualEvaluationResult"` // 互评结果
}
