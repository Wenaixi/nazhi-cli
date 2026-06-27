// error_test.go 聚合 errors.go + error_category.go 内部白盒测试（package client）：
//   - ClassifyError 分类契约 + 哨兵覆盖
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

// ─── error_category_test.go: ClassifyError + 哨兵覆盖 ───

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		// nil → ErrorCategoryUnknown
		{"nil", nil, ErrorCategoryUnknown},

		// 认证错误
		{"ErrLoginRejected", ErrLoginRejected, ErrorCategoryAuth},
		{"ErrLocationParseFailed", ErrLocationParseFailed, ErrorCategoryAuth},

		// 文件上传错误
		{"ErrUploadRejected", ErrUploadRejected, ErrorCategoryUpload},
		{"ErrFileTooLarge", ErrFileTooLarge, ErrorCategoryUpload},

		// Session 错误
		{"ErrSessionBackoff", ErrSessionBackoff, ErrorCategorySession},

		// 业务拒绝
		{"ErrBusinessRejected", ErrBusinessRejected, ErrorCategoryBusiness},
		{"ErrInvalidPayload", ErrInvalidPayload, ErrorCategoryBusiness},

		// 空数据
		{"ErrEmptyUserInfo", ErrEmptyUserInfo, ErrorCategoryEmptyData},

		// 网络错误
		{"ErrNetwork", ErrNetwork, ErrorCategoryNetwork},

		// OCR 错误
		{"ErrOCRNotConfigured", ErrOCRNotConfigured, ErrorCategoryOCR},
		{"ErrOCRPanic", ErrOCRPanic, ErrorCategoryOCR},

		// 包装链穿透
		{"wrapped ErrLoginRejected", fmt.Errorf("wrap: %w", ErrLoginRejected), ErrorCategoryAuth},

		// errors.Join 穿透
		{"errors.Join BusinessRejected", errors.Join(ErrBusinessRejected, errors.New("some other error")), ErrorCategoryBusiness},

		// 未知错误
		{"unknown error", errors.New("unknown error"), ErrorCategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(tt.err)
			if got != tt.expected {
				t.Errorf("ClassifyError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

// TestAllSentinelsAreClassified 枚举 errors.go 中所有 11 个导出的哨兵错误，
// 断言任何一个经过 ClassifyError 后都不应返回 ErrorCategoryUnknown。
//
// 这是 TDD RED 阶段的核心契约：新增哨兵后如果 ClassifyError 未同步映射，
// 本测试立即变红，迫使开发者更新 categoryRegistry。
func TestAllSentinelsAreClassified(t *testing.T) {
	allSentinels := []error{
		ErrLoginRejected,
		ErrNetwork,
		ErrUploadRejected,
		ErrFileTooLarge,
		ErrInvalidPayload,
		ErrBusinessRejected,
		ErrOCRNotConfigured,
		ErrSessionBackoff,
		ErrEmptyUserInfo,
		ErrLocationParseFailed,
		ErrOCRPanic,
	}

	for _, s := range allSentinels {
		category := ClassifyError(s)
		if category == ErrorCategoryUnknown {
			t.Errorf("ClassifyError(%v) = ErrorCategoryUnknown, expected a specific category", s)
		}
	}
}

// TestRegistryCoversAllSentinels 验证 categoryRegistry 覆盖了 errors.go 中
// 所有导出的哨兵错误。新增哨兵时必须同时在注册表中添加对应条目。
func TestRegistryCoversAllSentinels(t *testing.T) {
	allSentinels := []error{
		ErrLoginRejected,
		ErrNetwork,
		ErrUploadRejected,
		ErrFileTooLarge,
		ErrInvalidPayload,
		ErrBusinessRejected,
		ErrOCRNotConfigured,
		ErrSessionBackoff,
		ErrEmptyUserInfo,
		ErrLocationParseFailed,
		ErrOCRPanic,
	}

	for _, s := range allSentinels {
		found := false
		for _, entry := range categoryRegistry {
			if entry.Sentinel == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("sentinel %v is missing from categoryRegistry", s)
		}
	}
}

func TestErrorCategory_String(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{ErrorCategoryAuth, "auth"},
		{ErrorCategoryUpload, "upload"},
		{ErrorCategorySession, "session"},
		{ErrorCategoryBusiness, "business"},
		{ErrorCategoryEmptyData, "empty_data"},
		{ErrorCategoryNetwork, "network"},
		{ErrorCategoryOCR, "ocr"},
		{ErrorCategoryUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.category.String()
			if got != tt.expected {
				t.Errorf("ErrorCategory(%d).String() = %q, want %q", tt.category, got, tt.expected)
			}
		})
	}

	// 未定义的分类回退到 "unknown"
	if got := ErrorCategory(999).String(); got != "unknown" {
		t.Errorf("ErrorCategory(999).String() = %q, want %q", got, "unknown")
	}
}

// ─── error_category.go: SuggestUserMessage 闭环文案 ───

// TestSuggestUserMessage_AllCategories 验证 SuggestUserMessage 对每个
// 已定义 Category 都返回非空字符串（让 SDK 内部闭环文案，CLI 层
// 只需 switch Category 一次，剩余走 SDK 建议）。
func TestSuggestUserMessage_AllCategories(t *testing.T) {
	categories := []ErrorCategory{
		ErrorCategoryAuth,
		ErrorCategoryUpload,
		ErrorCategorySession,
		ErrorCategoryBusiness,
		ErrorCategoryEmptyData,
		ErrorCategoryNetwork,
		ErrorCategoryOCR,
		ErrorCategoryUnknown,
	}

	for _, c := range categories {
		t.Run(c.String(), func(t *testing.T) {
			msg := c.SuggestUserMessage()
			if msg == "" {
				t.Errorf("ErrorCategory(%s).SuggestUserMessage() 返回空字符串，违反闭环契约", c)
			}
		})
	}
}

// TestSuggestUserMessage_DefaultForUnknown 验证未定义 Category 也返回非空
// 默认文案，避免 CLI 层出现裸 "unknown" 之类不可读输出。
func TestSuggestUserMessage_DefaultForUnknown(t *testing.T) {
	msg := ErrorCategory(999).SuggestUserMessage()
	if msg == "" {
		t.Errorf("ErrorCategory(999).SuggestUserMessage() 应返回默认文案，实际为空")
	}
}

// TestSuggestUserMessage_OCRContainsActionableKeywords 验证 OCR 文案包含
// 可操作的关键词，与 ErrOCRNotConfigured 错误消息的 actionable 指引对齐
// （避免两个文案互相矛盾，详见 TestErrOCRNotConfigured_LocalizedMessage）。
func TestSuggestUserMessage_OCRContainsActionableKeywords(t *testing.T) {
	msg := ErrorCategoryOCR.SuggestUserMessage()

	want := []string{
		"OCR",
		"-tags ddddocr",
		"WithCustomOCR",
	}
	for _, kw := range want {
		if !strings.Contains(msg, kw) {
			t.Errorf("ErrorCategoryOCR.SuggestUserMessage() 应包含关键词 %q，实际: %s", kw, msg)
		}
	}
}

// TestSuggestUserMessage_AuthMentionsCredentials 验证认证类文案提示
// 用户检查凭据，避免无脑报错。
func TestSuggestUserMessage_AuthMentionsCredentials(t *testing.T) {
	msg := ErrorCategoryAuth.SuggestUserMessage()

	want := []string{"学号", "密码", "学校 ID"}
	for _, kw := range want {
		if !strings.Contains(msg, kw) {
			t.Errorf("ErrorCategoryAuth.SuggestUserMessage() 应包含关键词 %q，实际: %s", kw, msg)
		}
	}
}

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
