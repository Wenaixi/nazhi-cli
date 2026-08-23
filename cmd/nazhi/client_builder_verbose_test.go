// client_builder_verbose_test.go 锚定
// --verbose flag 联动 SDK logger 级别，让 c.logDebug 输出可见。
package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// TestBuildClientOpts_Verbose_SetsDebugLogger 验证 verbose 与 log-level 的联动：
// - verbose=true 且未显式传 log-level 时应为 Debug
// - verbose=false 默认应为 Warn（不启用 Debug）
// - 显式 --log-level 时 verbose 不应覆盖
func TestBuildClientOpts_Verbose_SetsDebugLogger(t *testing.T) {
	origVerbose := verbose
	origLevel := cliLogLevel
	origFile := cliLogFile
	origQuiet := quiet
	defer func() {
		verbose = origVerbose
		cliLogLevel = origLevel
		cliLogFile = origFile
		quiet = origQuiet
	}()

	newCmd := func() *cobra.Command {
		c := &cobra.Command{Use: "test"}
		c.Flags().String("token", "", "")
		c.Flags().String("base-url", "", "")
		c.Flags().String("sso-base", "", "")
		c.Flags().Int("timeout", 5, "")
		return c
	}

	quiet = false
	cliLogFile = ""

	// verbose=true 且无显式 level => Debug
	verbose = true
	cliLogLevel = ""
	t.Setenv("NAZHI_LOG_LEVEL", "")
	cmd1 := newCmd()
	opts1, _, err1 := buildClientOpts(cmd1, "base", "NAZHI_TIMEOUT", false)
	if err1 != nil {
		t.Fatalf("buildClientOpts with verbose=true 失败: %v", err1)
	}
	c1, _ := client.New(opts1...)
	if !c1.Enabled(context.Background(), slog.LevelDebug) {
		t.Errorf("verbose=true 时 logger 应启用 Debug")
	}

	// verbose=false 默认 => Warn（不启用 Debug）
	verbose = false
	cliLogLevel = ""
	cmd2 := newCmd()
	opts2, _, err2 := buildClientOpts(cmd2, "base", "NAZHI_TIMEOUT", false)
	if err2 != nil {
		t.Fatalf("buildClientOpts with verbose=false 失败: %v", err2)
	}
	c2, _ := client.New(opts2...)
	if c2.Enabled(context.Background(), slog.LevelDebug) {
		t.Errorf("verbose=false 时 logger 不应启用 Debug（应为 Warn）")
	}

	// 显式 --log-level info 时 verbose 不应覆盖为 Debug
	verbose = true
	cliLogLevel = "info"
	cmd3 := newCmd()
	opts3, _, err3 := buildClientOpts(cmd3, "base", "NAZHI_TIMEOUT", false)
	if err3 != nil {
		t.Fatalf("buildClientOpts with verbose+log-level 失败: %v", err3)
	}
	c3, _ := client.New(opts3...)
	if c3.Enabled(context.Background(), slog.LevelDebug) {
		t.Errorf("显式 --log-level info 时 verbose 不应覆盖为 Debug")
	}
	if !c3.Enabled(context.Background(), slog.LevelInfo) {
		t.Errorf("显式 --log-level info 时 logger 应启用 Info")
	}
}
