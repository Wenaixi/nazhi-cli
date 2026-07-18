package types

import "time"

// BonusInfo 积分信息（来自 getMonthBonusByStudentId 接口）。
type BonusInfo struct {
	ID          int64     `json:"id"`          // 积分记录 ID
	StudentID   int64     `json:"studentId"`   // 学生 ID
	StudentName string    `json:"studentName"` // 学生姓名
	ClassName   string    `json:"className"`   // 班级名称
	GradeName   string    `json:"gradeName"`   // 年级名称
	Score       float64   `json:"score"`       // 积分分值
	TermID      int64     `json:"termId"`      // 学期 ID
	TermName    string    `json:"termName"`    // 学期名称
	Month       string    `json:"month"`       // 月份
	CreateTime  time.Time `json:"createTime"`  // 创建时间
}

// BonusRank 积分排名（来自 getMonthBonusRankByClassId 接口）。
type BonusRank struct {
	StudentID   int64   `json:"studentId"`   // 学生 ID
	StudentName string  `json:"studentName"` // 学生姓名
	ClassName   string  `json:"className"`   // 班级名称
	Score       float64 `json:"score"`       // 积分分值
	Rank        int     `json:"rank"`        // 排名
}

// BonusDetail 积分明细（来自 getMonthBonusDetailByStudentId 接口）。
type BonusDetail struct {
	ID          int64     `json:"id"`          // 明细 ID
	StudentID   int64     `json:"studentId"`   // 学生 ID
	TypeName    string    `json:"typeName"`    // 积分类型名称
	Score       float64   `json:"score"`       // 积分分值
	Description string    `json:"description"` // 描述
	CreateTime  time.Time `json:"createTime"`  // 创建时间
}
