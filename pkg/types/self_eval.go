package types

// SelfEvalStatus 是自我评价状态。
type SelfEvalStatus struct {
	ID             int64  `json:"id"`
	StudentComment string `json:"studentComment"`
	TeacherComment string `json:"teacherComment"`
}
