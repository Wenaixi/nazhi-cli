package types

// UserUpdateInput 是用户信息更新的结构化输入。
//
// 对齐前端 modifyBox.vue：只暴露用户可编辑的 v-model 字段；
// SDK 将中文友好值转为 API 数字代码。零值/空串跳过（密码除外）。
//
// 用户应填：Telephone、FamilyAddress、Hobbies、GenderName、YouthLeague、
// NationName、IdCardType、IDCard、Birthday 或 BirthdayStr、StudentUuid、Seat。
//
// 非用户编辑（前端 disabled 或仅回显，一般勿填）：
// Name/StudentNumber 页面整包回传用；NationalStudentNumber 只读，
// UpdateMyInfoStructured 故意不写入该键，避免误改学籍。
type UserUpdateInput struct {
	// 可选高级：姓名/学号（前端非主编辑项；空则不发送）
	Name          string `json:"name,omitempty"`
	StudentNumber string `json:"studentNumber,omitempty"`
	// NationalStudentNumber 前端只读。字段保留兼容旧调用，但 Structured 路径忽略。
	NationalStudentNumber string `json:"nationalStudentNumber,omitempty"`

	// 用户可编辑
	Telephone     string `json:"telephone,omitempty"`
	FamilyAddress string `json:"familyAddress,omitempty"`
	Hobbies       string `json:"hobbies,omitempty"`

	// 个人信息（SDK 自动转换中文→API 代码）
	GenderName  string `json:"genderName,omitempty"`  // "男" / "女"
	YouthLeague string `json:"youthLeague,omitempty"` // "是" / "否"
	NationName  string `json:"nationName,omitempty"`
	IdCardType  string `json:"idCardType,omitempty"`
	IDCard      string `json:"idCard,omitempty"`

	// Birthday 对应前端 updateMyInfo 的 birthday 键。前端修改页实际发送 ISO 8601 UTC
	// 字符串；SDK 原样透传，不做日期或时区转换。
	Birthday string `json:"birthday,omitempty"`
	// BirthdayStr 兼容旧 SDK 调用；当 Birthday 为空时，其值写入 birthday。
	BirthdayStr string `json:"birthdayStr,omitempty"`

	// 密码（studentUuid；空串表示不修改）
	StudentUuid string `json:"studentUuid,omitempty"`

	// 座号（0 表示跳过——字面 "0"/0 同样视为跳过不发送，而前端整表 stringify 会原样发
	// seat:0；如需强制清零请走 UpdateMyInfo 裸 map 路径。FlexInt 兼容前端 el-input
	// 回显的字符串形态）
	Seat FlexInt `json:"seat,omitempty"`
}

// UserInfo 是用户个人资料的精简核心视图。
//
// 字段裁剪原则：
//   - 只保留业务 API 实际消费的核心身份/学校/班级字段，以及前端 getMyInfo 响应的原始字段（omitempty）
//   - 联系方式、证件号、积分等敏感或运营字段按需保留，避免不必要的 PII 暴露面
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

	// 座号（班级场景高频使用）
	Seat FlexInt `json:"seat"`

	// 前端 getMyInfo 响应的完整字段（omitempty，零值/null 时不在 JSON 输出中出现）

	// 联系方式
	Telephone string `json:"telephone,omitempty"` // 电话号码

	// 个人信息
	GenderName      string `json:"genderName,omitempty"`      // 性别名称
	BirthdayStr     string `json:"birthdayStr,omitempty"`     // 生日字符串
	YouthLeagueFlag FlexInt `json:"youthLeagueFlag,omitempty"` // 团员标志
	Nation          FlexInt `json:"nation,omitempty"`          // 民族（数字 1=汉族 等）
	FamilyAddress   string `json:"familyAddress,omitempty"`   // 家庭地址
	Hobbies         string `json:"hobbies,omitempty"`         // 爱好
	IDCard          string `json:"idCard,omitempty"`          // 身份证号
	IDType          FlexInt `json:"idType,omitempty"`          // 证件类型（数字）

	// 前端 getMyInfo 响应的额外原始字段
	RegistrationNumber string `json:"registrationNumber,omitempty"` // 中考报名号
	// AdmissionDate 入学日期数组。平台返回 JSON number 列表（如 [2025,9,1]），
	// 不是字符串列表；用 IntList 双兼容，避免解码失败被当成空用户。
	AdmissionDate IntList `json:"admissionDate,omitempty"`
	StudentUuid   string  `json:"studentUuid,omitempty"` // 学生 UUID / 密码
}
