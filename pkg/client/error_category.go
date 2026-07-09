// Package client 实现纳智综合评价目标平台的完整 Go SDK。
package client

import (
	"context"
	"errors"
	"net"
	"net/url"
)

// ErrorCategory 是错误分类枚举。
type ErrorCategory int

const (
	CategoryUnknown        ErrorCategory = iota
	CategoryContextCancel                // 上下文取消
	CategoryContextTimeout               // 上下文超时
	CategoryNetworkTimeout               // 网络层超时（TCP/TLS/响应头）
	CategoryBusinessError                // 业务拒绝（ErrBusinessRejected）
)

// String 实现 fmt.Stringer。
func (c ErrorCategory) String() string {
	switch c {
	case CategoryContextCancel:
		return "ContextCancel"
	case CategoryContextTimeout:
		return "ContextTimeout"
	case CategoryNetworkTimeout:
		return "NetworkTimeout"
	case CategoryBusinessError:
		return "BusinessError"
	default:
		return "Unknown"
	}
}

// ClassifyError 对错误进行分类，返回 ErrorCategory 枚举。
//
// 检查优先级（短路）：
//  1. context.Canceled → CategoryContextCancel
//  2. context.DeadlineExceeded → CategoryContextTimeout
//  3. url.Error.Timeout() / net.OpError.Timeout() → CategoryNetworkTimeout
//  4. errors.Is(err, ErrBusinessRejected) → CategoryBusinessError
//  5. 其他 → CategoryUnknown
func ClassifyError(err error) ErrorCategory {
	if errors.Is(err, context.Canceled) {
		return CategoryContextCancel
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return CategoryContextTimeout
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return CategoryNetworkTimeout
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) && netErr.Timeout() {
		return CategoryNetworkTimeout
	}
	if errors.Is(err, ErrBusinessRejected) {
		return CategoryBusinessError
	}
	return CategoryUnknown
}

// isContextError 判断是否为上下文取消或超时错误。
//
// isContextError 是 ClassifyError 的便捷封装，用于上下文取消/超时错误判定。
func isContextError(err error) bool {
	cat := ClassifyError(err)
	return cat == CategoryContextCancel || cat == CategoryContextTimeout
}

// isTimeoutError 检测错误是否为超时相关错误（不包含上下文取消）。
//
// isTimeoutError 是 ClassifyError 的便捷封装，用于超时错误判定。
// 注意：上下文取消（context.Canceled）不是超时，不纳入检查——取消由
// isContextError 或调用方自行处理。
func isTimeoutError(err error) bool {
	cat := ClassifyError(err)
	return cat == CategoryNetworkTimeout || cat == CategoryContextTimeout
}
