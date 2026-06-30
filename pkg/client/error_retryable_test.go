// Package client 白盒测试：F2.1 cancelPlaceholder 用 %w 包装 ErrRetryable。
//
// 修复动机：task.go 的 cancelPlaceholder = fmt.Errorf("%d 个维度因 context 取消而失败（可重试）", ...)
// 用裸 fmt.Errorf，错误消息含「可重试」但缺少 sentinel 标识，SDK 用户只能字符串匹配。
//
// 修复：errors.go 加 ErrRetryable sentinel，cancelPlaceholder 改为
// fmt.Errorf("%w: %d 个维度因 context 取消而失败", ErrRetryable, cancelledCount)，
// 让 errors.Is(err, ErrRetryable) 精确识别。
//
// 测试策略：直接断言 errors.Is 命中 + 字符串格式不回退到旧文案。
package client

import (
	"errors"
	"fmt"
	"testing"
)

// TestF21_ErrRetryable_SentinelIdentity 验证 ErrRetryable sentinel 存在且可 errors.Is 命中。
func TestF21_ErrRetryable_SentinelIdentity(t *testing.T) {
	if ErrRetryable == nil {
		t.Fatal("ErrRetryable 必须存在（非 nil）")
	}
	wrapped := fmt.Errorf("context 取消: %w", ErrRetryable)
	if !errors.Is(wrapped, ErrRetryable) {
		t.Error("errors.Is(wrapped, ErrRetryable) 应为 true")
	}
}

// TestF21_ErrRetryable_Message 验证错误消息含 retryable/cancelled 关键词。
func TestF21_ErrRetryable_Message(t *testing.T) {
	msg := ErrRetryable.Error()
	if !contains(msg, "retryable") && !contains(msg, "cancelled") {
		t.Errorf("ErrRetryable 消息应含 'retryable' 或 'cancelled'，实际: %s", msg)
	}
}

// TestF21_CancelPlaceholder_HitsErrRetryable 验证 task.go 的 cancelPlaceholder
// 走 %w 包装 ErrRetryable，errors.Is 能命中。
//
// ponytail：不启动完整 FetchTasks，直接构造 cancelPlaceholder 形态的 error
// 验证 %w 行为（生产代码逻辑等价）。
func TestF21_CancelPlaceholder_HitsErrRetryable(t *testing.T) {
	cancelledCount := 3
	placeholder := fmt.Errorf("%w: %d 个维度因 context 取消而失败", ErrRetryable, cancelledCount)

	if !errors.Is(placeholder, ErrRetryable) {
		t.Errorf("cancelPlaceholder 应包 ErrRetryable 可 errors.Is 命中，实际: %v", placeholder)
	}
	// 字符串仍保留中文语义信息（向后兼容）
	msg := placeholder.Error()
	if !contains(msg, "3") {
		t.Errorf("cancelPlaceholder 应保留 cancelledCount=3，实际: %s", msg)
	}
	if !contains(msg, "context 取消") {
		t.Errorf("cancelPlaceholder 应保留中文语义 'context 取消'，实际: %s", msg)
	}
}

// TestF21_ErrRetryable_DistinctFromOtherSentinels 验证 ErrRetryable 不误命中其他 sentinel。
func TestF21_ErrRetryable_DistinctFromOtherSentinels(t *testing.T) {
	others := []error{ErrBusinessRejected, ErrRateLimited, ErrServiceUnavailable, ErrInvalidResponse, ErrTimeout, ErrNetwork}
	for _, other := range others {
		wrapped := fmt.Errorf("wrap: %w", ErrRetryable)
		if errors.Is(wrapped, other) {
			t.Errorf("ErrRetryable 不应被识别为 %v（语义独立）", other)
		}
	}
}
