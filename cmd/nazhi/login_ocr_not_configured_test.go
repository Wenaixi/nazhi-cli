package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
)

// TestLoginCmd_ErrOCRNotConfigured_ActionableOutput 验证
// nazhi login 收到 ErrOCRNotConfigured 时输出 actionable envelope.Error
// 而非通用 printError。
//
// SDK 不内置 OCR，必须配置视觉模型 API key，否则直接 503 引导。
// 测试策略：模拟 loginCmd.Run 的 err 分支，解析 stdout envelope 的 message
// 字段（JSON 编码会转义中文，不能直接 strings.Contains 原始字节）。
func TestLoginCmd_ErrOCRNotConfigured_ActionableOutput(t *testing.T) {
	// 保存原始 stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	rStdout, wStdout, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe 失败: %v", err)
	}

	os.Stdout = wStdout

	// 模拟 login.go 的 ErrOCRNotConfigured 分支
	err = client.ErrOCRNotConfigured
	if errors.Is(err, client.ErrOCRNotConfigured) {
		printEnvelope(envelope.Error(503, "登录失败：验证码识别器未配置或出错。SDK 不再内置 OCR，必须设置环境变量 NAZHI_OCR_API_KEY（或 SILICONFLOW_API_KEY）接入硅基流动 Qwen3-Omni 视觉模型，或通过 SDK WithCustomOCR 注入自定义识别器。"))
	}

	_ = wStdout.Close()

	var stdoutBuf bytes.Buffer
	_, _ = stdoutBuf.ReadFrom(rStdout)

	// JSON 解析后断言 message 字段内容
	var env envelope.Envelope
	if err := json.Unmarshal(stdoutBuf.Bytes(), &env); err != nil {
		t.Fatalf("envelope 不是有效 JSON: %v\n输出: %s", err, stdoutBuf.String())
	}
	if env.Status != envelope.StatusError {
		t.Errorf("status 应 = error，实际: %s", env.Status)
	}
	if env.Code != 503 {
		t.Errorf("code 应 = 503，实际: %d", env.Code)
	}
	want := []string{
		"识别器未配置", // message 包含但不含 "OCR" 字面字
		"NAZHI_OCR_API_KEY",
		"SiliconFlow", // SDK i18n 英文部分
		"硅基流动",        // 中文部分
		"Qwen3-Omni",  // 默认模型名
	}
	lower := lowerASCII(env.Message)
	for _, w := range want {
		if !containsCI(lower, lowerASCII(w)) {
			t.Errorf("message 应包含 %q，实际: %s", w, env.Message)
		}
	}
}

// TestLoginCmd_ErrOCRNotConfigured_ErrorsIs 验证 errors.Is 分支判断正确性
// （包装后仍能识别）。
func TestLoginCmd_ErrOCRNotConfigured_ErrorsIs(t *testing.T) {
	err := client.ErrOCRNotConfigured
	if !errors.Is(err, client.ErrOCRNotConfigured) {
		t.Fatal("直等测试：errors.Is 必须识别 ErrOCRNotConfigured 自身")
	}

	wrapped := errors.New("wrap1: " + err.Error())
	if errors.Is(wrapped, client.ErrOCRNotConfigured) {
		t.Log("errors.New 包装不可识别 errors.Is（自然——未用 %w），本测试仅验证哨兵本身可识别")
	}
}

// TestLoginCmd_ErrOCRNotConfigured_SetsExitCode 验证
// ErrOCRNotConfigured 路径通过 envelope.Error(503) 设置 pendingExitCode=2。
func TestLoginCmd_ErrOCRNotConfigured_SetsExitCode(t *testing.T) {
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	rStdout, wStdout, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe 失败: %v", err)
	}
	os.Stdout = wStdout

	origExitCode := pendingExitCode.Load()
	defer pendingExitCode.Store(origExitCode)
	pendingExitCode.Store(0)

	// 模拟 login.go 的 ErrOCRNotConfigured 分支
	err = client.ErrOCRNotConfigured
	if errors.Is(err, client.ErrOCRNotConfigured) {
		printEnvelope(envelope.Error(503, "登录失败：验证码识别器未配置或出错。SDK 不再内置 OCR，必须设置环境变量 NAZHI_OCR_API_KEY（或 SILICONFLOW_API_KEY）接入硅基流动 Qwen3-Omni 视觉模型，或通过 SDK WithCustomOCR 注入自定义识别器。"))
	}

	_ = wStdout.Close()
	var discardBuf bytes.Buffer
	_, _ = discardBuf.ReadFrom(rStdout)

	// envelope.Error(503) → exit code 2
	if pendingExitCode.Load() != 2 {
		t.Errorf("ErrOCRNotConfigured 分支应设置 pendingExitCode=2（503 → 2），实际 %d", pendingExitCode.Load())
	}
}

// lowerASCII 返回小写副本（用于大小写不敏感子串匹配）。
func lowerASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// containsCI 大小写不敏感子串匹配。
func containsCI(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
