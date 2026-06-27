package client

import "errors"

// ErrorCategory 是错误分类枚举，用于 CLI 层按类别统一处理错误。
//
// 与哨兵错误的关系：
//   - 哨兵错误（errors.go）是 SDK 的错误契约，用于 errors.Is 精确匹配
//   - ErrorCategory 是粗粒度的分类，专用于 CLI 层 switch 分支
//   - 两者独立：new sentinel 只需加哨兵 + 在 ClassifyError 中映射即可，
//     无需修改 CLI 层代码
type ErrorCategory int

const (
	// ErrorCategoryAuth 登录/认证错误（ErrLoginRejected, ErrLocationParseFailed 等）。
	ErrorCategoryAuth ErrorCategory = iota

	// ErrorCategoryUpload 文件上传错误（ErrUploadRejected, ErrFileTooLarge 等）。
	ErrorCategoryUpload

	// ErrorCategorySession Session 激活错误（ErrSessionBackoff 等）。
	ErrorCategorySession

	// ErrorCategoryBusiness 业务拒绝错误（ErrBusinessRejected 等）。
	ErrorCategoryBusiness

	// ErrorCategoryEmptyData 空数据错误（ErrEmptyUserInfo 等）。
	ErrorCategoryEmptyData

	// ErrorCategoryNetwork 网络错误（ErrNetwork 等）。
	ErrorCategoryNetwork

	// ErrorCategoryOCR OCR 错误（ErrOCRNotConfigured, ErrOCRPanic 等）。
	ErrorCategoryOCR

	// ErrorCategoryUnknown 未知错误（无法匹配任何已知哨兵的错误）。
	ErrorCategoryUnknown
)

// String 返回 ErrorCategory 的可读名称。
func (c ErrorCategory) String() string {
	switch c {
	case ErrorCategoryAuth:
		return "auth"
	case ErrorCategoryUpload:
		return "upload"
	case ErrorCategorySession:
		return "session"
	case ErrorCategoryBusiness:
		return "business"
	case ErrorCategoryEmptyData:
		return "empty_data"
	case ErrorCategoryNetwork:
		return "network"
	case ErrorCategoryOCR:
		return "ocr"
	case ErrorCategoryUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// ClassifyError 根据 errors.Is 匹配哨兵错误，返回对应的 ErrorCategory。
//
// 实现策略：
//   - 先检查 nil → ErrorCategoryUnknown（避免 errors.Is(nil, nil) 的语义问题）
//   - 按类别分组检查，每个类别用 errors.Is 匹配所有相关的哨兵
//   - errors.Is 能穿透 fmt.Errorf("...%w...", sentinel) 和 errors.Join(...)
//   - 如果没有任何哨兵匹配，返回 ErrorCategoryUnknown
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	// 认证错误
	if errors.Is(err, ErrLoginRejected) || errors.Is(err, ErrLocationParseFailed) {
		return ErrorCategoryAuth
	}

	// 文件上传错误
	if errors.Is(err, ErrUploadRejected) || errors.Is(err, ErrFileTooLarge) {
		return ErrorCategoryUpload
	}

	// Session 错误
	if errors.Is(err, ErrSessionBackoff) {
		return ErrorCategorySession
	}

	// 业务拒绝
	if errors.Is(err, ErrBusinessRejected) {
		return ErrorCategoryBusiness
	}

	// 空数据
	if errors.Is(err, ErrEmptyUserInfo) {
		return ErrorCategoryEmptyData
	}

	// 网络错误
	if errors.Is(err, ErrNetwork) {
		return ErrorCategoryNetwork
	}

	// OCR 错误
	if errors.Is(err, ErrOCRNotConfigured) || errors.Is(err, ErrOCRPanic) {
		return ErrorCategoryOCR
	}

	return ErrorCategoryUnknown
}
