package types

// BonusInfo 积分信息（来自 getMonthBonusByStudentId 接口）。
//
// v2.0.0 变更：CreateTime 改为 string，保留服务端原始格式。
type BonusInfo struct {
	ID          int64   `json:"id"`
	StudentID   int64   `json:"studentId"`
	StudentName string  `json:"studentName"`
	ClassName   string  `json:"className"`
	GradeName   string  `json:"gradeName"`
	Score       float64 `json:"score"`
	TermID      int64   `json:"termId"`
	TermName    string  `json:"termName"`
	Month       string  `json:"month"`
	CreateTime  string  `json:"createTime"`
}

// BonusRank 积分排名（来自 getMonthBonusRankByClassId 接口）。
type BonusRank struct {
	StudentID   int64   `json:"studentId"`
	StudentName string  `json:"studentName"`
	ClassName   string  `json:"className"`
	Score       float64 `json:"score"`
	Rank        int     `json:"rank"`
}

// BonusDetail 积分明细（来自 getMonthBonusDetailByStudentId 接口）。
type BonusDetail struct {
	ID          int64   `json:"id"`
	StudentID   int64   `json:"studentId"`
	TypeName    string  `json:"typeName"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
	CreateTime  string  `json:"createTime"`
}
