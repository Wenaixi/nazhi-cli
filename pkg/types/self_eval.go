package types

// SelfEvalStatus 是自我评价状态。
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
