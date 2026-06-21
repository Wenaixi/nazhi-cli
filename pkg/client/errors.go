// Package client 实现纳智综合评价目标平台的完整 Go SDK。
//
// 每个 Client 实例拥有独立的 HTTP cookie jar，天然并发安全。
// 所有方法都需要 context.Context，支持超时与取消。
package client

import "errors"

// ─── 哨兵错误 ───

var (
	// ErrLoginRejected 登录被拒绝（凭证无效或验证码错误）。
	ErrLoginRejected = errors.New("login rejected: invalid credentials or captcha")

	// ErrTokenExpired token 已过期。
	ErrTokenExpired = errors.New("token expired")

	// ErrSessionExpired session 已过期。
	ErrSessionExpired = errors.New("session expired")

	// ErrNetwork 网络错误（连接超时、DNS 解析失败等）。
	ErrNetwork = errors.New("network error")

	// ErrUploadRejected 文件上传被服务器拒绝。
	ErrUploadRejected = errors.New("upload rejected by server")

	// ErrFileTooLarge 文件超出允许大小。
	ErrFileTooLarge = errors.New("file exceeds max size")

	// ErrInvalidPayload 任务请求体格式错误。
	ErrInvalidPayload = errors.New("invalid task payload")

	// ErrIncompleteResponse 响应缺少必要字段。
	ErrIncompleteResponse = errors.New("incomplete response from server")

	// ErrUnexpectedStatus 响应状态不符合预期。
	ErrUnexpectedStatus = errors.New("unexpected response status")
)
