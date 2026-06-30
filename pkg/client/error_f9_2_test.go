// F9.2: HTTP 状态码专用 sentinel 测试。
//
// 修复：errors.go 原本只有「业务拒绝」「网络错误」两类通用 sentinel，
// 429 / 5xx / 超时 / 非 200 响应都需要独立的 sentinel 让 SDK 用户
// 能通过 errors.Is 精确识别原因，决定「重试」「退避」「报错」等动作。
//
// 约束：sentinel 必须在 errors.go 内 public，可被 errors.Is 命中；
// 错误消息须包含语义关键词（rate limited / unavailable / timeout / invalid response）。
package client

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestF92_ErrRateLimited_SentinelIdentity 验证 ErrRateLimited sentinel 存在且可 errors.Is 识别。
func TestF92_ErrRateLimited_SentinelIdentity(t *testing.T) {
	if ErrRateLimited == nil {
		t.Fatal("ErrRateLimited 必须存在（非 nil）")
	}
	wrapped := fmt.Errorf("请求被限流: %w", ErrRateLimited)
	if !errors.Is(wrapped, ErrRateLimited) {
		t.Error("errors.Is(wrapped, ErrRateLimited) 应为 true")
	}
}

// TestF92_ErrRateLimited_Message 验证错误消息含语义关键词（让 SDK 用户能直接读 .Error()）。
func TestF92_ErrRateLimited_Message(t *testing.T) {
	msg := ErrRateLimited.Error()
	if !strings.Contains(strings.ToLower(msg), "rate limited") {
		t.Errorf("ErrRateLimited 消息应含 'rate limited'，实际: %s", msg)
	}
	if !strings.Contains(msg, "429") {
		t.Errorf("ErrRateLimited 消息应含 HTTP 状态码 429，实际: %s", msg)
	}
}

// TestF92_ErrServiceUnavailable_SentinelIdentity 验证 ErrServiceUnavailable sentinel 存在。
func TestF92_ErrServiceUnavailable_SentinelIdentity(t *testing.T) {
	if ErrServiceUnavailable == nil {
		t.Fatal("ErrServiceUnavailable 必须存在（非 nil）")
	}
	wrapped := fmt.Errorf("服务暂不可用: %w", ErrServiceUnavailable)
	if !errors.Is(wrapped, ErrServiceUnavailable) {
		t.Error("errors.Is(wrapped, ErrServiceUnavailable) 应为 true")
	}
}

// TestF92_ErrServiceUnavailable_Message 验证错误消息含 5xx 关键词。
func TestF92_ErrServiceUnavailable_Message(t *testing.T) {
	msg := ErrServiceUnavailable.Error()
	if !strings.Contains(strings.ToLower(msg), "unavailable") {
		t.Errorf("ErrServiceUnavailable 消息应含 'unavailable'，实际: %s", msg)
	}
}

// TestF92_ErrTimeout_SentinelIdentity 验证 ErrTimeout sentinel 存在。
func TestF92_ErrTimeout_SentinelIdentity(t *testing.T) {
	if ErrTimeout == nil {
		t.Fatal("ErrTimeout 必须存在（非 nil）")
	}
	wrapped := fmt.Errorf("请求超时: %w", ErrTimeout)
	if !errors.Is(wrapped, ErrTimeout) {
		t.Error("errors.Is(wrapped, ErrTimeout) 应为 true")
	}
}

// TestF92_ErrTimeout_Message 验证错误消息含 timeout 关键词。
func TestF92_ErrTimeout_Message(t *testing.T) {
	msg := ErrTimeout.Error()
	if !strings.Contains(strings.ToLower(msg), "timeout") {
		t.Errorf("ErrTimeout 消息应含 'timeout'，实际: %s", msg)
	}
}

// TestF92_ErrInvalidResponse_SentinelIdentity 验证 ErrInvalidResponse sentinel 存在。
func TestF92_ErrInvalidResponse_SentinelIdentity(t *testing.T) {
	if ErrInvalidResponse == nil {
		t.Fatal("ErrInvalidResponse 必须存在（非 nil）")
	}
	wrapped := fmt.Errorf("服务端返回异常状态: %w", ErrInvalidResponse)
	if !errors.Is(wrapped, ErrInvalidResponse) {
		t.Error("errors.Is(wrapped, ErrInvalidResponse) 应为 true")
	}
}

// TestF92_ErrInvalidResponse_Message 验证错误消息含 invalid response 关键词。
func TestF92_ErrInvalidResponse_Message(t *testing.T) {
	msg := ErrInvalidResponse.Error()
	if !strings.Contains(strings.ToLower(msg), "invalid response") {
		t.Errorf("ErrInvalidResponse 消息应含 'invalid response'，实际: %s", msg)
	}
}

// TestF92_SentinelsDistinct 验证 4 个 sentinel 彼此不互相命中（语义边界清晰）。
func TestF92_SentinelsDistinct(t *testing.T) {
	sentinels := []error{ErrRateLimited, ErrServiceUnavailable, ErrTimeout, ErrInvalidResponse}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			wrapped := fmt.Errorf("wrap %d: %w", i, a)
			if errors.Is(wrapped, b) {
				t.Errorf("sentinel[%d]=%v 错误命中 sentinel[%d]=%v（应仅 errors.Is 自身）", i, a, j, b)
			}
		}
	}
}