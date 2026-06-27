// Package types 内部白盒测试。
package types

import (
	"encoding/json"
	"errors"
	"testing"
)

// ─── DecodeDataList 补充测试 ───

// TestDecodeDataList_EmptyArray 验证空数组 [] 返回空切片（而非 nil）。
func TestDecodeDataList_EmptyArray(t *testing.T) {
	raw := json.RawMessage(`[]`)
	resp := UnifiedResponse{Code: 1, DataList: &raw}
	v, err := DecodeDataList[string](resp)
	if err != nil {
		t.Fatalf("空数组不应 err: %v", err)
	}
	if v == nil {
		t.Fatal("空数组应返回空切片，而非 nil")
	}
	if len(v) != 0 {
		t.Fatalf("空数组应返回 len=0，实际 len=%d", len(v))
	}
}

// ─── CheckCode 补充测试 ───

// TestCheckCode_Code0 验证 code=0 返回 *BusinessError{Code:0}。
func TestCheckCode_Code0(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: 0, Msg: strPtr("失败")})
	if err == nil {
		t.Fatal("code=0 应返回 error")
	}
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("应返回可 errors.As 的 *BusinessError，实际类型 %T", err)
	}
	if bizErr.Code != 0 {
		t.Errorf("Code 期望 0，实际 %d", bizErr.Code)
	}
	if bizErr.Msg != "失败" {
		t.Errorf("Msg 期望 '失败'，实际 %q", bizErr.Msg)
	}
}

// TestCheckCode_CodeNeg1 验证 code=-1 返回 *BusinessError{Code:-1}。
func TestCheckCode_CodeNeg1(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: -1, Msg: strPtr("系统错误")})
	if err == nil {
		t.Fatal("code=-1 应返回 error")
	}
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("应返回可 errors.As 的 *BusinessError，实际类型 %T", err)
	}
	if bizErr.Code != -1 {
		t.Errorf("Code 期望 -1，实际 %d", bizErr.Code)
	}
	if bizErr.Msg != "系统错误" {
		t.Errorf("Msg 期望 '系统错误'，实际 %q", bizErr.Msg)
	}
}

// TestCheckCode_ErrorContainsMsg 验证 BusinessError.Error() 输出包含 msg。
func TestCheckCode_ErrorContainsMsg(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: 400, Msg: strPtr("参数错误")})
	if err == nil {
		t.Fatal("code=400 应返回 error")
	}
	errMsg := err.Error()
	if bizErr, ok := err.(*BusinessError); ok {
		if bizErr.Msg != "参数错误" {
			t.Errorf("Msg 期望 '参数错误'，实际 %q", bizErr.Msg)
		}
	}
	if errMsg != "业务错误 (code=400): 参数错误" {
		t.Errorf("Error() 输出不符: %q", errMsg)
	}
}

// ─── BirthdayDate 补充测试 ───

// TestBirthdayDate_UnmarshalJSON_RFC3339 验证 "2009-12-11T00:00:00Z" 解析。
func TestBirthdayDate_UnmarshalJSON_RFC3339(t *testing.T) {
	var b BirthdayDate
	if err := b.UnmarshalJSON([]byte(`"2009-12-11T00:00:00Z"`)); err != nil {
		t.Fatalf("解析 RFC3339 失败: %v", err)
	}
	if b.Year != 2009 || b.Month != 12 || b.Day != 11 {
		t.Errorf("生日错: %d-%d-%d", b.Year, b.Month, b.Day)
	}
}

// TestBirthdayDate_UnmarshalJSON_InvalidString 验证无效字符串返回错误。
func TestBirthdayDate_UnmarshalJSON_InvalidString(t *testing.T) {
	var b BirthdayDate
	if err := b.UnmarshalJSON([]byte(`"not-a-date"`)); err == nil {
		t.Fatal("无效字符串应返回 error")
	}
}

// TestBirthdayDate_UnmarshalJSON_InvalidArray 验证无效数组返回错误。
func TestBirthdayDate_UnmarshalJSON_InvalidArray(t *testing.T) {
	var b BirthdayDate
	// 数组长度不足 3
	if err := b.UnmarshalJSON([]byte(`[2009]`)); err == nil {
		t.Fatal("长度不足的数组应返回 error")
	}
}
