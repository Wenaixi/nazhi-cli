package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ─── honor list / typical-case list 先校后建 ───

// runRealListCmd 用真实 honorListCmd / typicalCaseListCmd 的 Run 回调执行，
// 传入最小 flags 副本（honor_defaults_test.go:57 同款模式）。缺 token +
// 坏分页参数时，「先校后建」应首报分页错误（400/exit3），不落到
// buildBizClient 的 --token 必填。
func runRealListCmd(t *testing.T, run func(*cobra.Command, []string), extraFlags map[string]string) (string, int32) {
	t.Helper()
	cmd := &cobra.Command{Use: "list"}
	cmd.SetContext(context.Background())
	cmd.Flags().Int("page", 1, "")
	cmd.Flags().Int("page-size", 10, "")
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("base-url", "", "")
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("key", "", "")
	cmd.Flags().Int("status", 3, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")
	_ = cmd.Flags().Set("base-url", "http://127.0.0.1:1")
	// 不设 token：buildBizClient 会报「--token 必填」
	for k, v := range extraFlags {
		_ = cmd.Flags().Set(k, v)
	}

	originalQuiet, originalVerbose := quiet, verbose
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	defer func() {
		quiet, verbose = originalQuiet, originalVerbose
		pendingExitCode.Store(0)
	}()

	stdout, _, restore := captureStdio(t)
	run(cmd, nil)
	restore()
	return stdout.String(), pendingExitCode.Load()
}

// TestHonorList_ValidateBeforeBuildClient 锁定：honor list 缺
// token + page=0 时首报分页错误，不落「--token 必填」——脚本自查不再
// 把分页错误误判为鉴权问题。
func TestHonorList_ValidateBeforeBuildClient(t *testing.T) {
	out, code := runRealListCmd(t, honorListCmd.Run, map[string]string{"page": "0", "page-size": "0"})
	if code != 3 {
		t.Errorf("缺 token + page=0 应走参数错误退出码 3（分页校验优先），实际 %d", code)
	}
	if !strings.Contains(out, "--page 与 --page-size 必须为正整数") {
		t.Errorf("应先报分页参数错误（先校后建），实际: %s", out)
	}
	if strings.Contains(out, "--token 必填") {
		t.Errorf("不应走到 buildBizClient 的 --token 必填（校验后移会先报鉴权错误）: %s", out)
	}
}

// TestTypicalCaseList_ValidateBeforeBuildClient 同款锁定 typical-case list。
func TestTypicalCaseList_ValidateBeforeBuildClient(t *testing.T) {
	out, code := runRealListCmd(t, typicalCaseListCmd.Run, map[string]string{"page": "0", "page-size": "0"})
	if code != 3 {
		t.Errorf("缺 token + page=0 应走参数错误退出码 3（分页校验优先），实际 %d", code)
	}
	if !strings.Contains(out, "--page 与 --page-size 必须为正整数") {
		t.Errorf("应先报分页参数错误（先校后建），实际: %s", out)
	}
	if strings.Contains(out, "--token 必填") {
		t.Errorf("不应走到 buildBizClient 的 --token 必填（校验后移会先报鉴权错误）: %s", out)
	}
}
