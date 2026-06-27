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

// SuggestUserMessage 返回面向 CLI 用户的建议文案。
//
// 设计动机: 让分类系统在 SDK 内部闭环——caller 不必每次手写 5 行文案。
// CLI 层只 switch Category 一次，剩余文案走 SDK 建议。
//
// 注意:
//   - 文案使用简体中文，与 CLI 当前风格一致。i18n 后续可考虑，但当前最小实现只支持中文。
//   - ErrorCategoryOCR 的文案与 ErrOCRNotConfigured 错误消息的 actionable
//     关键词对齐，避免 SDK 两处文案互相矛盾（详见 error_test.go）。
func (c ErrorCategory) SuggestUserMessage() string {
	switch c {
	case ErrorCategoryAuth:
		return "登录被服务器拒绝。请检查学号/密码/学校 ID 是否正确，或确认 SSO 服务端是否正常。"
	case ErrorCategoryUpload:
		return "文件上传失败。请确认文件大小/格式是否符合要求，或稍后重试。"
	case ErrorCategorySession:
		return "Session 激活失败后处于冷却期，请稍后重试。"
	case ErrorCategoryBusiness:
		return "业务操作失败。请检查请求参数是否合法，或稍后重试。"
	case ErrorCategoryEmptyData:
		return "服务器返回空数据。请稍后重试或确认账号权限。"
	case ErrorCategoryNetwork:
		return "网络请求失败。请检查网络连接或稍后重试。"
	case ErrorCategoryOCR:
		return "OCR 识别器未配置。当前构建未启用 -tags ddddocr，请下载预编译 release 二进制（nazhi-cli releases 页面），或通过 SDK 调 client.WithCustomOCR(myRecognizer) 注入识别器。"
	case ErrorCategoryUnknown:
		return "未知错误。"
	default:
		return "未分类错误。"
	}
}

// sentinelEntry 是哨兵错误到 ErrorCategory 的映射条目。
type sentinelEntry struct {
	Sentinel error
	Category ErrorCategory
}

// categoryRegistry 是哨兵错误到 ErrorCategory 的注册表。
// ClassifyError 遍历此注册表进行匹配，新增哨兵只需在此追加一行。
var categoryRegistry = []sentinelEntry{
	{ErrLoginRejected, ErrorCategoryAuth},
	{ErrLocationParseFailed, ErrorCategoryAuth},
	{ErrUploadRejected, ErrorCategoryUpload},
	{ErrFileTooLarge, ErrorCategoryUpload},
	{ErrSessionBackoff, ErrorCategorySession},
	{ErrBusinessRejected, ErrorCategoryBusiness},
	{ErrInvalidPayload, ErrorCategoryBusiness},
	{ErrEmptyUserInfo, ErrorCategoryEmptyData},
	{ErrNetwork, ErrorCategoryNetwork},
	{ErrOCRNotConfigured, ErrorCategoryOCR},
	{ErrOCRPanic, ErrorCategoryOCR},
}

// ClassifyError 根据 errors.Is 匹配哨兵错误，返回对应的 ErrorCategory。
//
// 实现策略：
//   - 先检查 nil → ErrorCategoryUnknown（避免 errors.Is(nil, nil) 的语义问题）
//   - 遍历 categoryRegistry 逐一匹配
//   - errors.Is 能穿透 fmt.Errorf("...%w...", sentinel) 和 errors.Join(...)
//   - 如果没有任何哨兵匹配，返回 ErrorCategoryUnknown
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	for _, e := range categoryRegistry {
		if errors.Is(err, e.Sentinel) {
			return e.Category
		}
	}

	return ErrorCategoryUnknown
}
