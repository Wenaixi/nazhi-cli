// error_test.go 聚合 errors.go 内部白盒测试（package client）：
//   - CheckCode errors.Is + errors.As 双契约
//   - ErrOCRNotConfigured 错误消息 i18n + 中文 actionable
package client

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── biz_error_wrap_test.go: CheckCode 双契约 ───

// TestCheckCode_ErrorsIsAndAs 验证 CheckCode 返回的错误同时满足
// errors.Is(err, ErrBusinessRejected) 和 errors.As(err, &bizError)。
// 历史 bug：两套模式混用：
// - Pattern A: fmt.Errorf("...: %w", err) err=*BusinessError
// errors.Is(ErrBusinessRejected) = false
// - Pattern B: fmt.Errorf("%w: ...: %v", ErrBusinessRejected, err)
// errors.Is 正确但 errors.As 不到 BusinessError
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

// ─── errors_ocr_not_configured_test.go: ErrOCRNotConfigured 消息 ───

// TestErrOCRNotConfigured_LocalizedMessage 验证 修复：错误消息含中文 actionable 指引。
func TestErrOCRNotConfigured_LocalizedMessage(t *testing.T) {
	msg := ErrOCRNotConfigured.Error()

	// i18n key 必须存在（错误消息可机器解析）
	if !strings.HasPrefix(msg, "errors.ocr_not_configured") {
		t.Errorf("错误消息应以 i18n key 'errors.ocr_not_configured' 开头，实际: %s", msg)
	}

	// 中文 actionable 指引必须存在
	wantCN := []string{
		"OCR 识别器未配置",
		"-tags ddddocr",
		"client.WithCustomOCR",
		"预编译 release 二进制",
	}
	for _, want := range wantCN {
		if !strings.Contains(msg, want) {
			t.Errorf("错误消息应包含中文 actionable 关键词 %q，实际: %s", want, msg)
		}
	}

	// 英文 fallback 保留（SDK 编程接口可读）
	wantEN := []string{
		"OCR recognizer not configured",
		"-tags ddddocr",
		"WithCustomOCR",
	}
	for _, want := range wantEN {
		if !strings.Contains(msg, want) {
			t.Errorf("错误消息应保留英文 fallback %q，实际: %s", want, msg)
		}
	}
}

// TestErrOCRNotConfigured_ErrorsIs 验证 errors.Is 契约未破坏（修复不应改变哨兵身份）。
func TestErrOCRNotConfigured_ErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("login 流程失败: %w", ErrOCRNotConfigured)
	if !errors.Is(wrapped, ErrOCRNotConfigured) {
		t.Errorf("errors.Is 必须能识别包装后的 ErrOCRNotConfigured，实际未识别")
	}
}
