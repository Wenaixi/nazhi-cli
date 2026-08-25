package main

import (
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
	cmd.SetContext(nil)
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
