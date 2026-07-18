package types

// DemocraticActivity 民主评价活动（来自 getActivity 接口）。
type DemocraticActivity struct {
	ID        int64  `json:"id"`        // 活动 ID
	Name      string `json:"name"`      // 活动名称
	SubPlanID int64  `json:"subPlanId"` // 子计划 ID
	EndDate   string `json:"endDate"`   // 结束日期
	StartDate string `json:"startDate"` // 开始日期
	Status    int    `json:"status"`    // 活动状态
	CreateTime string `json:"createTime"` // 创建时间（原始字符串）
}

// SelfEvaluationItem 自评项目（来自 getSelfEvaluation 接口）。
// 前端期望 snake_case JSON 键：content_detail / evaluation_result / evaluation_result2 / score / dimension_id / ifHasData。
type SelfEvaluationItem struct {
	ID               int64   `json:"id"`               // 项目 ID
	ContentDetail    string  `json:"content_detail"`   // 评价内容
	EvaluationResult string  `json:"evaluation_result"` // 评价结果
	EvaluationResult2 string `json:"evaluation_result2"` // 评价结果2
	Score            float64 `json:"score"`            // 分数
	DimensionID      int64   `json:"dimension_id"`     // 维度 ID
	IfHasData        int     `json:"ifHasData"`        // 是否有数据
}

// MutualEvaluation 互评记录（来自 getMutualEvaluationDetail 接口）。
//
// 注意：前端实际响应的 dataList 元素格式是 map 嵌套结构，
// 顶层含 student_name / student_id / data[]，data 内是评价项。
// 此类型仅做参考，实际使用 map[string]any 解析。
type MutualEvaluation struct {
	ID                     int64  `json:"id"`                     // 记录 ID
	StudentID              int64  `json:"student_id"`             // 学生 ID
	StudentName            string `json:"student_name"`           // 学生姓名
	StudentNumber          string `json:"student_number"`         // 学号
	MutualEvaluationResult string `json:"mutual_evaluation_result"` // 互评结果
	EvaluationStatus       int    `json:"evaluation_status"`      // 评价状态
}

// DemocraticResult 民主评价结果（来自 getDemocraticResult 接口）。
type DemocraticResult struct {
	SelfEvaluation    string `json:"selfEvaluation"`    // 自评成绩
	MutualEvaluation  string `json:"mutualEvaluation"`  // 互评成绩
	TeacherEvaluation string `json:"teacherEvaluation"` // 教师评价成绩
}

// MutualPersonInfo 互评人员信息（来自 getMutualPersonInfo 接口）。
type MutualPersonInfo struct {
	IfMutualPerson     bool          `json:"ifMutualPerson"`     // 是否是互评人
	EvaluatedNumbers    int          `json:"evaluatedNumbers"`    // 已评价人数
	NotEvaluatedNumbers int          `json:"notEvaluatedNumbers"` // 未评价人数
	ClassStudentList    []ClassStudent `json:"classStudentList"`  // 班级学生列表
}

// ClassStudent 班级学生信息。
// 前端期望 snake_case JSON 键：student_id / name / student_number / mutual_evaluation_result / no。
type ClassStudent struct {
	StudentID             int64  `json:"student_id"`              // 学生 ID
	Name                  string `json:"name"`                    // 学生姓名
	StudentNumber         string `json:"student_number"`          // 学号
	MutualEvaluationResult string `json:"mutual_evaluation_result"` // 互评结果
	No                    bool   `json:"no"`                      // 选中状态（前端用于标记选择）
}

// SelfEvaluationInput 自评提交的单项（AddOrUpdateSelfEvaluation 请求体数组元素）。
//
// 注意：前端提交时，根据 ifHasData 状态发送不同字段组合：
//   - 已有数据（ifHasData==1）：使用 id
//   - 新增数据（ifHasData!=1）：使用 quotaId + evaluationType + dimensionId
// 两个路径都含 activityId / evaluationResult / evaluationScore / score。
type SelfEvaluationInput struct {
	ActivityID       int64   `json:"activityId"`
	QuotaID          int64   `json:"quotaId,omitempty"`
	ID               int64   `json:"id,omitempty"`
	EvaluationResult int     `json:"evaluationResult"`
	EvaluationScore  float64 `json:"evaluationScore"`
	EvaluationType   int     `json:"evaluationType,omitempty"`
	DimensionID      int64   `json:"dimensionId,omitempty"`
	Score            float64 `json:"score"`
}

// MutualEvaluationInput 互评提交的完整请求体（AddOrUpdateMutualEvaluation 的请求体数组元素）。
type MutualEvaluationInput struct {
	StudentID   int64           `json:"student_id"`
	StudentName string          `json:"student_name"`
	Data        []MutualEvalItem `json:"data"`
}

// MutualEvalItem 互评中单个学生的评价项。
type MutualEvalItem struct {
	ActivityID       int64   `json:"activityId"`
	QuotaID          int64   `json:"quotaId,omitempty"`
	ID               int64   `json:"id,omitempty"`
	EvaluationResult int     `json:"evaluationResult"`
	EvaluationScore  float64 `json:"evaluationScore"`
	EvaluationType   int     `json:"evaluationType,omitempty"`
	DimensionID      int64   `json:"dimensionId,omitempty"`
	Score            float64 `json:"score"`
	StudentID        int64   `json:"studentId,omitempty"`
}

// DemocraticActivityListResult 民主评价活动分页结果。
type DemocraticActivityListResult struct {
	Records []DemocraticActivity `json:"records"`
	Page    *PageBean            `json:"page,omitempty"`
}
