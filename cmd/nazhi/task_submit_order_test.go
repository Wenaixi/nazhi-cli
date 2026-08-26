package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestTaskSubmitCmd_MissingPayloadTakesPrecedenceOverMissingToken 回归测试：
// task submit 与 task edit 校验顺序一致——payload 缺失先于 token 缺失报告。
//
// 背景（十三域审计 P2-E）：submit 原实现先 buildBizClient 后校验 payload，
// 双参数缺失时报 token 错误（printParamError→stderr）；edit 相反报 payload
// （printEnvelope→stdout）。同因不同果且通道分裂。统一为先校验 payload。
func TestTaskSubmitCmd_MissingPayloadTakesPrecedenceOverMissingToken(t *testing.T) {
	cmd := &cobra.Command{Use: "task-submit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	// 不设置 token
	cmd.Flags().String("base-url", "", "")
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	// 不设置 payload
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")

	quiet = false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdoutBuf, _, restore := captureStdio(t)
	taskSubmitCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()

	if !strings.Contains(stdout, "--payload 为必填") {
		t.Errorf("payload 缺失应优先于 token 缺失在 stdout 报告，实际: %q", stdout)
	}
}

// TestHonorAddCmd_MissingPayloadTakesPrecedenceOverMissingToken 回归测试：
// honor add 与 task submit/edit、typical-case、user update 的校验顺序一致——
// payload 缺失先于 token 缺失报告。十五域审计发现 honor add 先 buildBizClient
// 后校验 payload，双参数缺失时报「--token 为必填」而非「--payload 为必填」，
// 与同族七个命令的收敛规范相悖。
func TestHonorAddCmd_MissingPayloadTakesPrecedenceOverMissingToken(t *testing.T) {
	cmd := &cobra.Command{Use: "honor-add"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	// 不设置 token
	cmd.Flags().String("base-url", "", "")
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	// 不设置 payload

	quiet = false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdoutBuf, _, restore := captureStdio(t)
	honorAddCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()

	if !strings.Contains(stdout, "--payload 为必填") {
		t.Errorf("payload 缺失应优先于 token 缺失在 stdout 报告，实际: %q", stdout)
	}
}

// TestTaskPreviewCmd_MissingPayloadTakesPrecedenceOverMissingToken 回归测试：
// task preview 遵守与 submit/edit 相同的「先校验 payload 后建客户端」不变式。
// 十五域审计发现 preview 是该不变式的漏网第三处（submit/edit 已在十三域 P2-E 修复）。
func TestTaskPreviewCmd_MissingPayloadTakesPrecedenceOverMissingToken(t *testing.T) {
	cmd := &cobra.Command{Use: "task-preview"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	// 不设置 token
	cmd.Flags().String("base-url", "", "")
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	// 不设置 payload
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")
	cmd.Flags().Bool("edit", false, "")

	quiet = false
	pendingExitCode.Store(0)
	t.Cleanup(func() {
		pendingExitCode.Store(0)
		_ = closeAllClients()
	})

	stdoutBuf, _, restore := captureStdio(t)
	taskPreviewCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()

	if !strings.Contains(stdout, "--payload 为必填") {
		t.Errorf("payload 缺失应优先于 token 缺失在 stdout 报告，实际: %q", stdout)
	}
}
