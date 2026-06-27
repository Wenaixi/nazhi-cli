package client

import (
	"errors"
	"fmt"
	"testing"
)

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
