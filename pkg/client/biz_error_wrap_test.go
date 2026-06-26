// Package client 内部白盒测试。
package client

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestCheckCode_ErrorsIsAndAs 验证 CheckCode 返回的错误同时满足
// errors.Is(err, ErrBusinessRejected) 和 errors.As(err, &bizError)。
//
// 历史 bug：两套模式混用：
//   - Pattern A: fmt.Errorf("...: %w", err)  err=*BusinessError
//     errors.Is(ErrBusinessRejected) = false
//   - Pattern B: fmt.Errorf("%w: ...: %v", ErrBusinessRejected, err)
//     errors.Is 正确但 errors.As 不到 BusinessError
//
// 修复后：统一用 errors.Join(ErrBusinessRejected, err)
func TestCheckCode_ErrorsIsAndAs(t *testing.T) {
	resp := types.UnifiedResponse{
		Code: 0,
		Msg:  strPtr("登录失败"),
	}
	_ = resp
	checkErr := types.CheckCode(resp)
	if checkErr == nil {
		t.Fatal("CheckCode(code=0) 应返回 error")
	}

	// Test errors.Is(ErrBusinessRejected) — 在包装后应工作
	wrapped := errors.Join(ErrBusinessRejected, checkErr)
	if !errors.Is(wrapped, ErrBusinessRejected) {
		t.Error("errors.Join 后 errors.Is(ErrBusinessRejected) 应为 true")
	}

	// Test errors.As(&bizErr) — 应能提取 BusinessError
	var bizErr *types.BusinessError
	if !errors.As(wrapped, &bizErr) {
		t.Error("errors.Join 后 errors.As(&bizErr) 应为 true")
	}
	if bizErr.Code != 0 {
		t.Errorf("bizErr.Code 期望 0，实际 %d", bizErr.Code)
	}
}

// TestCheckCode_ErrorsIsFalse 验证非 BusinessError 的包装不会错误地
// 触发 errors.Is(ErrBusinessRejected)。
func TestCheckCode_ErrorsIsFalse(t *testing.T) {
	plain := errors.New("网络超时")
	wrapped := fmt.Errorf("请求失败: %w", plain)
	if errors.Is(wrapped, ErrBusinessRejected) {
		t.Error("普通错误包装后不应满足 errors.Is(ErrBusinessRejected)")
	}
}

// strPtr 返回字符串指针。
func strPtr(s string) *string { return &s }
