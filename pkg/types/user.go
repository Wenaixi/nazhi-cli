package types

// UserInfo 是用户个人资料的精简核心视图。
//
// 字段裁剪原则（v1.0.0 起）：
//   - 只保留业务 API 实际消费的核心身份/学校/班级字段
//   - 联系方式、证件号、积分等敏感或运营字段已移除，避免不必要的 PII 暴露面
//   - 生日/爱好/状态码等历史字段已移除
//
// v1.3.0 扩展：补齐前端 getMyInfo 响应的全部原始字段，新增字段使用 omitempty。
type UserInfo struct {
	// 基础身份
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	StudentNumber         string `json:"studentNumber"`         // 学号（平台登录主学号）
	StudentID             int64  `json:"studentId"`             // 学生 ID
	StudyNumber           string `json:"studyNumber"`           // 校内短学号（部分业务接口用）
	NationalStudentNumber string `json:"nationalStudentNumber"` // 全国学号

	// 学校 / 班级 / 年级
	SchoolID   int64  `json:"schoolId"`
	SchoolName string `json:"schoolName,omitempty"` // 平台返回 null 时省略
	GradeID    int64  `json:"gradeId"`
	GradeName  string `json:"gradeName"`
	ClassID    int64  `json:"classId"`
	ClassName  string `json:"className"`

	// 座号（恢复：班级场景高频需要）
	Seat int `json:"seat"`

	// v1.3.0 新增：前端 getMyInfo 响应的完整字段
	// 所有新字段使用 omitempty，零值/null 时不在 JSON 输出中出现。

	// 联系方式
	Telephone string `json:"telephone,omitempty"` // 电话号码

	// 个人信息
	GenderName        string `json:"genderName,omitempty"`        // 性别名称
	BirthdayStr       string `json:"birthdayStr,omitempty"`       // 生日字符串
	YouthLeagueFlag   int    `json:"youthLeagueFlag,omitempty"`   // 团员标志
	Nation            int    `json:"nation,omitempty"`            // 民族（数字 1=汉族 等）
	FamilyAddress     string `json:"familyAddress,omitempty"`     // 家庭地址
	Hobbies           string `json:"hobbies,omitempty"`           // 爱好
	IDCard            string `json:"idCard,omitempty"`              // 身份证号
	IDType            int    `json:"idType,omitempty"`            // 证件类型（数字）
}
