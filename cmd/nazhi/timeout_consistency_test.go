package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTimeoutTestCmd 构造带 timeout flag 的最小命令。
func newTimeoutTestCmd(flagValue string, flagSet bool) *cobra.Command {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Int("timeout", 15, "")
	if flagSet {
		_ = cmd.Flags().Set("timeout", flagValue)
	}
	return cmd
}

// TestResolveTimeoutSec_SameOutcomeForFlagAndEnv --timeout 0 与 NAZHI_TIMEOUT=0
// 是同一类非法输入，必须同果：回退注册默认 15 且都告警。此前 flag 非法告警后
// 回退 30、env 非法静默回退 30、未设置却是 15，三通道不一致。
func TestResolveTimeoutSec_SameOutcomeForFlagAndEnv(t *testing.T) {
	t.Setenv("NAZHI_TIMEOUT", "0")
	_, stderr, restore := captureStdio(t)
	got := resolveTimeoutSec(newTimeoutTestCmd("0", true), "NAZHI_TIMEOUT")
	restore()
	if got != 15 {
		t.Errorf("flag 非法应回退 15，实际 %d", got)
	}
	if !strings.Contains(stderr.String(), "warn") {
		t.Errorf("flag 非法应有告警，实际 stderr=%q", stderr.String())
	}

	_, stderr2, restore2 := captureStdio(t)
	gotEnv := resolveTimeoutSec(newTimeoutTestCmd("", false), "NAZHI_TIMEOUT")
	restore2()
	if gotEnv != 15 {
		t.Errorf("env 非法应回退 15，实际 %d", gotEnv)
	}
	if !strings.Contains(stderr2.String(), "warn") {
		t.Errorf("env 非法应有告警，此前是静默回退。实际 stderr=%q", stderr2.String())
	}
}

// TestResolveTimeoutSec_ValidPathsUnchanged 合法路径不受收敛影响：
// flag 显式值优先于 env；未设置时用注册默认 15；正数 env 生效。
func TestResolveTimeoutSec_ValidPathsUnchanged(t *testing.T) {
	t.Setenv("NAZHI_TIMEOUT", "")
	if got := resolveTimeoutSec(newTimeoutTestCmd("30", true), "NAZHI_TIMEOUT"); got != 30 {
		t.Errorf("显式 flag 应生效，实际 %d", got)
	}
	if got := resolveTimeoutSec(newTimeoutTestCmd("", false), "NAZHI_TIMEOUT"); got != 15 {
		t.Errorf("未设置应用注册默认 15，实际 %d", got)
	}

	t.Setenv("NAZHI_TIMEOUT", "45")
	if got := resolveTimeoutSec(newTimeoutTestCmd("", false), "NAZHI_TIMEOUT"); got != 45 {
		t.Errorf("合法 env 应生效，实际 %d", got)
	}
}
