// Package ocr 内部白盒测试：Pool 生命周期管理。
package ocr

import (
	"testing"
)

// TestPool_RecognizeAfterClose_ReturnsError 回归测试 F1（round-6）：
// Pool.Close 后调用 Recognize 必须返回错误，防止新 OCR 实例和 tempDir 泄漏。
//
// 历史问题：Pool.Close 通过 closeOnce 保证只排空一次 inits map，但 Close 后
// Recognize 仍能从 sync.Pool 获取/创建新 OCR 实例并 trackInit - 这些实例永远不会
// 被 Close 路径清理，导致 tempDir 永久泄漏到 %TEMP%。
//
// F1 修复：Recognize 入口处检查 Pool.closed 标记，已关闭则直接返回错误。
func TestPool_RecognizeAfterClose_ReturnsError(t *testing.T) {
	p := NewPool(0)

	// 先 Close 池
	if err := p.Close(); err != nil {
		t.Fatalf("首次 Close 应无错: %v", err)
	}

	// Close 后再调用 Recognize 必须返回错误
	_, err := p.Recognize([]byte("fake"))
	if err == nil {
		t.Fatal("Pool.Close 后调 Recognize 应返回错误，但返回 nil")
	}
	if err.Error() != "OCR pool is closed" {
		t.Errorf("错误消息应为「OCR pool is closed」，实际: %v", err)
	}
}

// TestPool_RecognizeAfterClose_NoNewInstanceLeak 验证 Pool.Close 后 Recognize
// 不会向 inits map 注册新实例（F1 的防御深度测试）。
//
// 即使 Recognize 被错误地调用，inits map 也不应增长。本测试先 Close，再调一次
// Recognize（预期返回错误），然后断言 inits map 长度仍为 0（无泄漏）。
func TestPool_RecognizeAfterClose_NoNewInstanceLeak(t *testing.T) {
	p := NewPool(0)

	// 先注册一个实例，确保 Close 路径正常
	o := &OCR{}
	p.trackInit(o)

	if err := p.Close(); err != nil {
		t.Fatalf("首次 Close 应无错: %v", err)
	}

	// Close 后调 Recognize → 应被拒绝
	_, _ = p.Recognize([]byte("fake"))

	// inits map 不应包含任何新注册的实例
	remaining := 0
	p.inits.Range(func(_, _ any) bool {
		remaining++
		return true
	})
	if remaining != 0 {
		t.Errorf("Close 后 Recognize 不应向 inits 注册新实例，剩余 %d 个", remaining)
	}
}
