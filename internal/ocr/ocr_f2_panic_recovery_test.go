// Package ocr 内部白盒测试：F2 initOnce panic recovery 状态泄漏。
//
// F2 bug：initOnce 的 defer recover 路径错误地把 o.initialized 标记为 true、
// 但 o.initErr 仍为 nil、o.ocr 仍为 nil，导致后续 Recognize 看到
//
//	initialized=true → 跳过 initOnce
//	initErr=nil → 跳过错误分支
//	ocr=nil → 报 "OCR unavailable: recognizer is nil"
//
// 原始 panic 根因丢失，*OCR 实例永久卡死。
//
// 修复后契约：defer recover 路径应保留根因（initErr 不为 nil）、
// 不标记 initialized，使后续 Recognize 重试 initOnce 并把根因上报。
package ocr

import (
	"strings"
	"testing"
)

// TestOCR_F2_PanicRecovery_PreservesInitErr 验证 panic recovery 路径：
//   - 返回 initErr（不重新 panic），包含原始 panic 信息
//   - o.initialized 保持 false（允许下次 initOnce 重试）
//   - o.initErr 不为 nil（保留根因）
//
// 注入方式：通过 testPanicHook 函数指针在 initOnce 内 ddddocr.New 之前
// 触发 panic，仅供测试使用。
func TestOCR_F2_PanicRecovery_PreservesInitErr(t *testing.T) {
	// 注入 panic：让 initOnce 在 ddddocr.New 之前抛 "simulated CGO panic"
	o := &OCR{testPanicHook: func() { panic("simulated CGO panic") }}

	err := o.initOnce()
	if err == nil {
		t.Fatal("expected error from initOnce after panic, got nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("expected error to contain 'panic' (preserve root cause), got: %v", err)
	}
	if o.initialized {
		t.Fatal("OCR should NOT be marked initialized after panic (allows retry)")
	}
	if o.initErr == nil {
		t.Fatal("initErr should be set to preserve panic root cause")
	}
	if o.ocr != nil {
		t.Fatal("o.ocr should remain nil after panic recovery")
	}
	if o.tempDir != "" {
		t.Fatal("o.tempDir should be cleared after panic recovery")
	}
}
