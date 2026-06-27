// Package client 内部白盒测试。
// 修复：ErrOCRNotConfigured 错误消息改为中文 actionable +
// i18n key 前缀。验证契约：
// - errors.Is(err, ErrOCRNotConfigured) 仍能精确识别（关键 SDK 契约）
// - 错误消息包含中文 actionable 指引（CLI 用户可见）
// - 错误消息包含 i18n key 前缀「errors.ocr_not_configured」
// - 英文 fallback 消息保留（SDK 编程接口可读性）
package client

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestErrOCRNotConfigured_LocalizedMessage 验证 修复：错误消息含中文 actionable 指引。
func TestErrOCRNotConfigured_LocalizedMessage(t *testing.T) {
	msg := ErrOCRNotConfigured.Error()

	// i18n key 必须存在（错误消息可机器解析）
	if !strings.HasPrefix(msg, "errors.ocr_not_configured") {
		t.Errorf("错误消息应以 i18n key 'errors.ocr_not_configured' 开头，实际: %s", msg)
	}

	// 中文 actionable 指引必须存在
	wantCN := []string{
		"OCR 识别器未配置",
		"-tags ddddocr",
		"client.WithCustomOCR",
		"预编译 release 二进制",
	}
	for _, want := range wantCN {
		if !strings.Contains(msg, want) {
			t.Errorf("错误消息应包含中文 actionable 关键词 %q，实际: %s", want, msg)
		}
	}

	// 英文 fallback 保留（SDK 编程接口可读）
	wantEN := []string{
		"OCR recognizer not configured",
		"-tags ddddocr",
		"WithCustomOCR",
	}
	for _, want := range wantEN {
		if !strings.Contains(msg, want) {
			t.Errorf("错误消息应保留英文 fallback %q，实际: %s", want, msg)
		}
	}
}

// TestErrOCRNotConfigured_ErrorsIs 验证 errors.Is 契约未破坏（修复不应改变哨兵身份）。
func TestErrOCRNotConfigured_ErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("login 流程失败: %w", ErrOCRNotConfigured)
	if !errors.Is(wrapped, ErrOCRNotConfigured) {
		t.Errorf("errors.Is 必须能识别包装后的 ErrOCRNotConfigured，实际未识别")
	}
}
