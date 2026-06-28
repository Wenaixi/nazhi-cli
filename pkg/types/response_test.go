// Package types 内部白盒测试。
package types

import (
	"errors"
	"testing"
)

// TestCheckCode_ReturnsBusinessErrorWithCode 回归测试：CheckCode 必须返回
// *BusinessError 让 errors.As 拿回 code 数值。
//
// 历史 bug：CheckCode 用 fmt.Errorf 把 code 嵌入错误信息字符串，
// 下游 errors.As 拿不到结构化 code，无法区分 code=2 vs code=500 vs code=999。
func TestCheckCode_ReturnsBusinessErrorWithCode(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: 500, Msg: strPtr("密码错误")})
	if err == nil {
		t.Fatal("code=500 应返回 error")
	}
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatalf("CheckCode 应返回可 errors.As 的 *BusinessError，实际类型 %T", err)
	}
	if bizErr.Code != 500 {
		t.Errorf("Code 错：%d", bizErr.Code)
	}
	if bizErr.Msg != "密码错误" {
		t.Errorf("Msg 错：%s", bizErr.Msg)
	}
}

// TestCheckCode_Code1ReturnsNil 验证成功码 1 不返回 error。
func TestCheckCode_Code1ReturnsNil(t *testing.T) {
	if err := CheckCode(UnifiedResponse{Code: 1, Msg: strPtr("ok")}); err != nil {
		t.Errorf("code=1 应返回 nil，实际 %v", err)
	}
}

// TestCheckCode_MissingMsgFallback 验证 msg 缺失时使用"未知错误"占位。
func TestCheckCode_MissingMsgFallback(t *testing.T) {
	err := CheckCode(UnifiedResponse{Code: 999})
	var bizErr *BusinessError
	if !errors.As(err, &bizErr) {
		t.Fatal("应返回 *BusinessError")
	}
	if bizErr.Msg != "未知错误" {
		t.Errorf("Msg 缺失时应 fallback '未知错误'，实际 %q", bizErr.Msg)
	}
}

func strPtr(s string) *string { return &s }
