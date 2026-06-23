// Package types 内部白盒测试。
package types

import (
	"encoding/json"
	"testing"
)

// TestBirthdayDate_UnmarshalJSON_Array 验证 [2009,12,11] 数组形式解析。
func TestBirthdayDate_UnmarshalJSON_Array(t *testing.T) {
	var b BirthdayDate
	if err := b.UnmarshalJSON([]byte(`[2009,12,11]`)); err != nil {
		t.Fatalf("解析数组失败: %v", err)
	}
	if b.Year != 2009 || b.Month != 12 || b.Day != 11 {
		t.Errorf("生日错: %d-%d-%d", b.Year, b.Month, b.Day)
	}
}

// TestBirthdayDate_UnmarshalJSON_String 验证 "2009-12-11" 字符串形式解析。
func TestBirthdayDate_UnmarshalJSON_String(t *testing.T) {
	var b BirthdayDate
	if err := b.UnmarshalJSON([]byte(`"2009-12-11"`)); err != nil {
		t.Fatalf("解析字符串失败: %v", err)
	}
	if b.Year != 2009 || b.Month != 12 || b.Day != 11 {
		t.Errorf("生日错: %d-%d-%d", b.Year, b.Month, b.Day)
	}
}

// TestBirthdayDate_UnmarshalJSON_Null 验证 null 解析为零值。
func TestBirthdayDate_UnmarshalJSON_Null(t *testing.T) {
	var b BirthdayDate
	if err := b.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Errorf("null 解析应成功: %v", err)
	}
}

// TestBirthdayDate_String 验证 String() 输出格式。
func TestBirthdayDate_String(t *testing.T) {
	b := BirthdayDate{Year: 2009, Month: 12, Day: 11}
	if got := b.String(); got != "2009-12-11" {
		t.Errorf("String() 错: %q", got)
	}
}

// TestUserInfo_BirthdayDualForm 回归测试：UserInfo 同时支持 birthday 数组
// 和 birthdayStr 字符串双形态。
//
// 历史 bug：ef5c1ad 把 Birthday 从 ac9e084 的自定义 UnmarshalJSON
// 回退到普通 string + birthdayStr json tag，若 server 改返回 [y,m,d]
// 数组会反序列化失败。
func TestUserInfo_BirthdayDualForm(t *testing.T) {
	// 场景 1: server 返回 birthday 数组
	resp1 := `{
		"id": 12345,
		"name": "张三",
		"birthday": [2009, 12, 11]
	}`
	var u1 UserInfo
	if err := json.Unmarshal([]byte(resp1), &u1); err != nil {
		t.Fatalf("场景1 解析失败: %v", err)
	}
	if u1.BirthdayDate == nil || u1.BirthdayDate.Year != 2009 {
		t.Errorf("场景1: BirthdayDate 应解析 [2009,12,11]，实际 %+v", u1.BirthdayDate)
	}
	if u1.Birthday != "" {
		t.Errorf("场景1: Birthday 字符串应为空，实际 %q", u1.Birthday)
	}

	// 场景 2: server 返回 birthdayStr 字符串
	resp2 := `{
		"id": 12345,
		"name": "张三",
		"birthdayStr": "2009-12-11 00:00:00"
	}`
	var u2 UserInfo
	if err := json.Unmarshal([]byte(resp2), &u2); err != nil {
		t.Fatalf("场景2 解析失败: %v", err)
	}
	if u2.Birthday != "2009-12-11 00:00:00" {
		t.Errorf("场景2: Birthday 字符串应保留，实际 %q", u2.Birthday)
	}
	if u2.BirthdayDate != nil {
		t.Errorf("场景2: BirthdayDate 应为 nil，实际 %+v", u2.BirthdayDate)
	}
}
