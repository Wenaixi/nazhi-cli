// Package types_test 包含 nazhi-cli types 包的单元测试。
package types_test

import (
	"encoding/json"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── 测试: Birthday 解析 ───

// TestBirthday_Array 验证 JSON 数组形式 [year, month, day] 的解析（HAR 实测形态）。
func TestBirthday_Array(t *testing.T) {
	data := []byte(`{"birthday":[2009,12,11]}`)
	var u types.UserInfo
	if err := json.Unmarshal(data, &u); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if u.Birthday.Year != 2009 || u.Birthday.Month != 12 || u.Birthday.Day != 11 {
		t.Errorf("数组形式解析错误: 期望 2009-12-11, 得到 %d-%d-%d",
			u.Birthday.Year, u.Birthday.Month, u.Birthday.Day)
	}
	if u.Birthday.YMD() != "2009-12-11" {
		t.Errorf("YMD 格式错误: 期望 2009-12-11, 得到 %s", u.Birthday.YMD())
	}
	if u.Birthday.String() != "2009-12-11" {
		t.Errorf("String 格式错误: 期望 2009-12-11, 得到 %s", u.Birthday.String())
	}
}

// TestBirthday_String 验证 JSON 字符串形式 "2009-12-11" 的解析。
func TestBirthday_String(t *testing.T) {
	data := []byte(`{"birthday":"2009-12-11"}`)
	var u types.UserInfo
	if err := json.Unmarshal(data, &u); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if u.Birthday.Year != 2009 || u.Birthday.Month != 12 || u.Birthday.Day != 11 {
		t.Errorf("字符串形式解析错误: 期望 2009-12-11, 得到 %d-%d-%d",
			u.Birthday.Year, u.Birthday.Month, u.Birthday.Day)
	}
}

// TestBirthday_DateTimeString 验证日期时间字符串的解析（如 "2009-12-11 00:00:00"）。
func TestBirthday_DateTimeString(t *testing.T) {
	data := []byte(`{"birthday":"2009-12-11 00:00:00"}`)
	var u types.UserInfo
	if err := json.Unmarshal(data, &u); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if u.Birthday.Year != 2009 || u.Birthday.Month != 12 || u.Birthday.Day != 11 {
		t.Errorf("日期时间字符串解析错误: 期望 2009-12-11, 得到 %d-%d-%d",
			u.Birthday.Year, u.Birthday.Month, u.Birthday.Day)
	}
	if u.Birthday.Str != "2009-12-11 00:00:00" {
		t.Errorf("原始字符串丢失: 期望 '2009-12-11 00:00:00', 得到 %q", u.Birthday.Str)
	}
}

// TestBirthday_Null 验证 null 的处理（不报错，字段为零值）。
func TestBirthday_Null(t *testing.T) {
	data := []byte(`{"birthday":null}`)
	var u types.UserInfo
	if err := json.Unmarshal(data, &u); err != nil {
		t.Fatalf("null 解析失败: %v", err)
	}
	if !u.Birthday.IsZero() {
		t.Errorf("null 应该是 IsZero=true, 得到 %+v", u.Birthday)
	}
	if u.Birthday.YMD() != "" {
		t.Errorf("null YMD 应为空, 得到 %q", u.Birthday.YMD())
	}
}

// TestBirthday_MarshalJSON 验证序列化输出 [year, month, day] 数组形式。
func TestBirthday_MarshalJSON(t *testing.T) {
	b := types.Birthday{Year: 2009, Month: 12, Day: 11}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	want := `[2009,12,11]`
	if string(data) != want {
		t.Errorf("序列化格式错误: 期望 %s, 得到 %s", want, string(data))
	}
}

// TestBirthday_MarshalJSON_Zero 验证零值序列化为 null。
func TestBirthday_MarshalJSON_Zero(t *testing.T) {
	b := types.Birthday{}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("零值应序列化为 null, 得到 %s", string(data))
	}
}

// TestBirthday_RoundTrip 验证 UserInfo 完整往返序列化。
func TestBirthday_RoundTrip(t *testing.T) {
	src := types.UserInfo{
		ID:    32USER_ID_REDACTED,
		Name:  "张三",
		Seat:  29,
		ClassName: "高一八班",
		Birthday: types.Birthday{Year: 2009, Month: 12, Day: 11},
	}
	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var dst types.UserInfo
	if err := json.Unmarshal(data, &dst); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if dst.Birthday.Year != 2009 || dst.Birthday.Month != 12 || dst.Birthday.Day != 11 {
		t.Errorf("往返后生日错误: 期望 2009-12-11, 得到 %d-%d-%d",
			dst.Birthday.Year, dst.Birthday.Month, dst.Birthday.Day)
	}
	if dst.Name != "张三" || dst.Seat != 29 {
		t.Errorf("其他字段丢失: %+v", dst)
	}
}
