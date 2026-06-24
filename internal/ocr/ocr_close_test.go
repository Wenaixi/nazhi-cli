// Package ocr 内部白盒测试。
package ocr

import (
	"strings"
	"testing"
)

// TestOCR_CloseReturnsRemoveAllError 回归测试：Close() 必须返回 os.RemoveAll
// 的清理错误，不能静默吞掉（Windows AV 持锁 / Linux 权限拒绝 / 任意 platform
// 错误都应让调用方知情）。
//
// 历史 bug：Close() 无条件 return nil，导致 temp dir 永久泄漏到 %TEMP%。
func TestOCR_CloseReturnsRemoveAllError(t *testing.T) {
	// 用 null byte 路径让 os.RemoveAll 必失败（跨平台、零依赖）
	o := &OCR{tempDir: "\x00invalid-tempdir-for-test"}

	err := o.Close()
	if err == nil {
		t.Fatal("Close() 应返回 os.RemoveAll 错误，但返回 nil")
	}
	if !strings.Contains(err.Error(), "清理临时目录") {
		t.Errorf("错误信息应说明 '清理临时目录' 失败，实际：%v", err)
	}
}

// TestOCR_Close_ResetsInitState 回归测试：Close() 必须清除初始化副作用，
// 使后续 Recognize() 重新走初始化路径（或者返回明确的"已关闭"错误），
// 而不是带着陈旧的 once.done=true + o.ocr=nil 状态调用 Classification。
//
// 该测试已经被 TestOCR_RecognizeAfterClose_ReturnsError 覆盖（同一个 bug 路径：
// once.Do 完成 + o.ocr=nil → Recognize 触发 nil panic）。保留本测试作为
// "Close() 后再调用不应 panic" 的最小冒烟测试，覆盖 "Close 没产生 once 副作用
// 但仍有调用风险" 的退化场景。
func TestOCR_Close_ResetsInitState(t *testing.T) {
	o := &OCR{tempDir: "\x00invalid-tempdir-for-test"}

	if err := o.Close(); err == nil {
		t.Fatal("Close 应返回错误（null byte 路径）")
	}

	// Close 后再调 Recognize 不能 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close() 后再调 Recognize() panic: %v", r)
		}
	}()
	_, _ = o.Recognize([]byte("fake"))
}

// TestOCR_RecognizeAfterCloseViaOnce_ReturnsError 直接复现 close 触发的 panic 路径：
// once 已 done（不再执行 init 闭包） + o.ocr=nil（被 Close 清空） +
// 调 Recognize → 当前实现在 o.ocr.Classification 触发 nil pointer panic。
func TestOCR_RecognizeAfterCloseViaOnce_ReturnsError(t *testing.T) {
	o := &OCR{}

	// 模拟「曾经成功初始化」：直接把 initialized 标记为 true，跳过 init 闭包，
	// 然后设置 o.ocr=nil 完美复现 Close 后的状态
	o.initMu.Lock()
	o.initialized = true
	o.ocr = nil
	o.initMu.Unlock()

	// 关键断言：必须返回错误，绝不能 panic
	var result string
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Close() 后调 Recognize() 触发 panic: %v", r)
			}
		}()
		result, err = o.Recognize([]byte("fake-image-bytes"))
	}()

	if err == nil {
		t.Fatal("Close() 后调 Recognize() 应返回错误，但返回 nil 且 result=" + result)
	}
}
