package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// UserInfo 是用户个人资料的精简核心视图。
//
// 字段裁剪原则（v1.0.0 起）：
//   - 只保留业务 API 实际消费的核心身份/学校/班级字段
//   - 联系方式、证件号、积分等敏感或运营字段已移除，避免不必要的 PII 暴露面
//   - 生日/爱好/状态码/座号等历史字段已移除，如有需要可后续按需补回
type UserInfo struct {
	// 基础身份
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	StudentNumber string `json:"studentNumber"` // 学号
	StudentID     int64  `json:"studentId"`     // 学生 ID

	// 学校 / 班级 / 年级
	SchoolID   int64  `json:"schoolId"`
	SchoolName string `json:"schoolName,omitempty"` // 平台返回 null 时省略
	GradeID    int64  `json:"gradeId"`
	GradeName  string `json:"gradeName"`
	ClassID    int64  `json:"classId"`
	ClassName  string `json:"className"`
}

// BirthdayDate 生日结构体（兼容 [2009,12,11] 数组和 "2009-12-11" 字符串）。
// 解决 ef5c1ad 移除 ac9e084 自定义解析后丢失的"双形态容错"能力——
// 若 server 升级只返回 birthday 数组（无 birthdayStr），BirthdayDate 仍可解析。
//
// 当前 UserInfo 未消费该类型，但保留供其他模块按需引用，避免删后回归。
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