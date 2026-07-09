// Package types 内部白盒测试。
package types

import (
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
