package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ─── circle comment / like / delete：先校后建（同款范式）───

// runCircleWriteCmd 用真实 circle 写命令的 Run 回调执行，传入最小 flags 副本。
// 缺 token + 非法参数时，「先校后建」应首报参数错误（400/exit 3），不落到
// buildBizClient 的 --token 必填。
func runCircleWriteCmd(t *testing.T, run func(*cobra.Command, []string), extraFlags map[string]string) (string, string, int32) {
	t.Helper()
	cmd := &cobra.Command{Use: "circle-write"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("id", "", "")
	cmd.Flags().String("content", "", "")
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("base-url", "", "")
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Bool("verbose", false, "")
	cmd.Flags().Bool("quiet", false, "")
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

	stdout, stderr, restore := captureStdio(t)
	run(cmd, nil)
	restore()
	return stdout.String(), stderr.String(), pendingExitCode.Load()
}

// TestCircleWrite_ValidateBeforeBuildClient 锁定同款契约：缺 token +
// 非法 --id 时首报参数错误，不落 buildBizClient 的 --token 必填。三个 circle
// 写命令此前均先 buildBizClient 再校验（先建后校），坏参数会被 --token 必填
func TestCircleWrite_ValidateBeforeBuildClient(t *testing.T) {
	cases := []struct {
		name string
		run  func(*cobra.Command, []string)
		id   string
		want string
	}{
		{"comment-empty-id", circleCommentCmd.Run, "", "--id 为必填"},
		{"comment-negative-id", circleCommentCmd.Run, "-5", "--id 必须为正整数"},
		{"like-empty-id", circleLikeCmd.Run, "", "--id 为必填"},
		{"like-negative-id", circleLikeCmd.Run, "-5", "--id 必须为正整数"},
		{"delete-empty-id", circleDeleteCmd.Run, "", "--id 为必填"},
		{"delete-negative-id", circleDeleteCmd.Run, "-5", "--id 必须为正整数"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, code := runCircleWriteCmd(t, tc.run, map[string]string{"id": tc.id})
			if code != 3 {
				t.Errorf("缺 token + %q id 应走参数错误退出码 3，实际 %d", tc.id, code)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Errorf("应先报 %q（先校后建），实际 stdout: %q", tc.want, stdout)
			}
			if strings.Contains(stdout, "--token 必填") {
				t.Errorf("不应走到 buildBizClient 的 --token 必填（校验后移会先报鉴权错误）: %q", stdout)
			}
		})
	}
}
