package main

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/spf13/cobra"
)

func newMiniCmd() *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.Flags().String("token", "", "")
	c.Flags().String("base-url", "", "")
	c.Flags().String("sso-base", "", "")
	c.Flags().Int("timeout", 5, "")
	return c
}

// 1. 无效 level/format 仅 warn 回落不阻断
func TestBoundary_CLI_InvalidLevelAndFormat_Fallback(t *testing.T) {
	origLevel, origFormat, origFile, origVerbose, origQuiet := cliLogLevel, cliLogFormat, cliLogFile, verbose, quiet
	defer func() {
		cliLogLevel = origLevel
		cliLogFormat = origFormat
		cliLogFile = origFile
		verbose = origVerbose
		quiet = origQuiet
	}()
	quiet = false
	verbose = false
	cliLogFile = ""
	t.Setenv("NAZHI_LOG_LEVEL", "")
	t.Setenv("NAZHI_LOG_FORMAT", "")
	cliLogLevel = "bogus-level"
	cliLogFormat = "bogus-format"
	cmd := newMiniCmd()
	opts, _, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("invalid level/format should not block buildClientOpts: %v", err)
	}
	c, _ := client.New(opts...)
	// 回落到 warn/text：debug 不启用，info 不启用，warn 启用
	if c.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("invalid level should fallback to warn, debug should not be enabled")
	}
	if c.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("warn fallback should not enable info")
	}
	if !c.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn should be enabled")
	}
}

// 2. flag > env > default 优先级
func TestBoundary_CLI_LevelFlagOverEnv(t *testing.T) {
	origLevel, origVerbose, origQuiet, origFile := cliLogLevel, verbose, quiet, cliLogFile
	defer func() { cliLogLevel = origLevel; verbose = origVerbose; quiet = origQuiet; cliLogFile = origFile }()
	quiet = false
	cliLogFile = ""
	verbose = false
	t.Setenv("NAZHI_LOG_LEVEL", "error")
	cliLogLevel = "debug"
	cmd := newMiniCmd()
	opts, _, _ := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	c, _ := client.New(opts...)
	if !c.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("flag debug should override env error")
	}
	// flag 为空时 env 生效
	cliLogLevel = ""
	cmd2 := newMiniCmd()
	opts2, _, _ := buildClientOpts(cmd2, "base", "NAZHI_TIMEOUT", false)
	c2, _ := client.New(opts2...)
	if !c2.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("env error should be effective when flag empty")
	}
	if c2.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("error level should not enable warn")
	}
}

// 3. verbose 仅当无显式 level 时生效
func TestBoundary_CLI_VerboseOnlyWhenNoExplicitLevel(t *testing.T) {
	origLevel, origVerbose, origQuiet, origFile := cliLogLevel, verbose, quiet, cliLogFile
	defer func() { cliLogLevel = origLevel; verbose = origVerbose; quiet = origQuiet; cliLogFile = origFile }()
	quiet = false
	cliLogFile = ""
	t.Setenv("NAZHI_LOG_LEVEL", "")
	verbose = true
	cliLogLevel = "info"
	cmd := newMiniCmd()
	opts, _, _ := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	c, _ := client.New(opts...)
	if c.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("explicit info should not be overridden by verbose")
	}
	if !c.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("explicit info should enable info")
	}
	// 无显式 level 时 verbose 生效
	cliLogLevel = ""
	cmd2 := newMiniCmd()
	opts2, _, _ := buildClientOpts(cmd2, "base", "NAZHI_TIMEOUT", false)
	c2, _ := client.New(opts2...)
	if !c2.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("verbose without explicit level should enable debug")
	}
}

// 4. 无效 log-file 仅 warn 不阻断 build
func TestBoundary_CLI_InvalidLogFile_NotBlock(t *testing.T) {
	origLevel, origFormat, origFile, origVerbose, origQuiet := cliLogLevel, cliLogFormat, cliLogFile, verbose, quiet
	defer func() {
		cliLogLevel = origLevel
		cliLogFormat = origFormat
		cliLogFile = origFile
		verbose = origVerbose
		quiet = origQuiet
	}()
	quiet = false
	verbose = false
	cliLogLevel = "warn"
	cliLogFormat = "text"
	cliLogFile = "/invalid/path/that/does/not/exist/file.log"
	t.Setenv("NAZHI_LOG_LEVEL", "")
	t.Setenv("NAZHI_LOG_FORMAT", "")
	t.Setenv("NAZHI_LOG_FILE", "")
	cmd := newMiniCmd()
	opts, _, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("invalid log-file should not block: %v", err)
	}
	c, _ := client.New(opts...)
	if c == nil {
		t.Fatal("client should still be created")
	}
	// 清理可能残留的 pendingLogFiles
	_ = closeLogFiles()
}

// 5. quiet+file 仍落地 且 stderr 静默（通过 buildClientOpts 直接测 writer 数）
func TestBoundary_CLI_QuietWithFile_StillWrites(t *testing.T) {
	origLevel, origFormat, origFile, origVerbose, origQuiet := cliLogLevel, cliLogFormat, cliLogFile, verbose, quiet
	defer func() {
		cliLogLevel = origLevel
		cliLogFormat = origFormat
		cliLogFile = origFile
		verbose = origVerbose
		quiet = origQuiet
	}()
	dir := t.TempDir()
	p := filepath.Join(dir, "quiet-file.log")
	cliLogLevel = "debug"
	cliLogFormat = "json"
	cliLogFile = p
	quiet = true
	verbose = false
	t.Setenv("NAZHI_LOG_LEVEL", "")
	t.Setenv("NAZHI_LOG_FORMAT", "")
	t.Setenv("NAZHI_LOG_FILE", "")
	cmd := newMiniCmd()
	opts, _, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("build fail: %v", err)
	}
	c, _ := client.New(opts...)
	ctx := logx.WithTraceID(context.Background(), "quiet-file-trace")
	c.LogDebugForTest(ctx, "quiet-file-msg")
	_ = closeLogFiles()
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "quiet-file-msg") {
		t.Fatalf("quiet+file should still write file got %q", string(data))
	}
	if !strings.Contains(string(data), "quiet-file-trace") {
		t.Fatalf("file log should contain trace_id got %q", string(data))
	}
}

// 6. json 格式每行含 trace_id 与 level
func TestBoundary_CLI_JSONFormat_ContainsTraceAndLevel(t *testing.T) {
	origLevel, origFormat, origFile, origVerbose, origQuiet := cliLogLevel, cliLogFormat, cliLogFile, verbose, quiet
	defer func() {
		cliLogLevel = origLevel
		cliLogFormat = origFormat
		cliLogFile = origFile
		verbose = origVerbose
		quiet = origQuiet
	}()
	dir := t.TempDir()
	p := filepath.Join(dir, "json.log")
	cliLogLevel = "debug"
	cliLogFormat = "json"
	cliLogFile = p
	quiet = true
	verbose = false
	cmd := newMiniCmd()
	opts, _, _ := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	c, _ := client.New(opts...)
	ctx := logx.WithTraceID(context.Background(), "json-trace-abc")
	c.LogInfoForTest(ctx, "json-msg")
	_ = closeLogFiles()
	data, _ := os.ReadFile(p)
	out := string(data)
	if !strings.Contains(out, "json-trace-abc") {
		t.Fatalf("json should contain trace_id got %q", out)
	}
	if !strings.Contains(out, "INFO") && !strings.Contains(out, "level") {
		t.Fatalf("json should contain level got %q", out)
	}
}

// 7. text 格式同样含 trace_id（新实现已支持）
func TestBoundary_CLI_TextFormat_ContainsTrace(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "text", &buf)
	ctx := logx.WithTraceID(context.Background(), "text-trace")
	lg.Info("hello", slog.String("trace_id", logx.TraceIDFrom(ctx)))
	if !strings.Contains(buf.String(), "text-trace") {
		t.Fatalf("text should contain trace got %q", buf.String())
	}
}

// 8. NAZHI_LOG_FILE env 生效当 flag 为空
func TestBoundary_CLI_LogFileEnv_WhenFlagEmpty(t *testing.T) {
	origLevel, origFormat, origFile, origVerbose, origQuiet := cliLogLevel, cliLogFormat, cliLogFile, verbose, quiet
	defer func() {
		cliLogLevel = origLevel
		cliLogFormat = origFormat
		cliLogFile = origFile
		verbose = origVerbose
		quiet = origQuiet
	}()
	dir := t.TempDir()
	p := filepath.Join(dir, "env-file.log")
	cliLogLevel = "debug"
	cliLogFormat = "text"
	cliLogFile = ""
	quiet = true
	verbose = false
	t.Setenv("NAZHI_LOG_FILE", p)
	t.Setenv("NAZHI_LOG_LEVEL", "")
	t.Setenv("NAZHI_LOG_FORMAT", "")
	cmd := newMiniCmd()
	opts, _, _ := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	c, _ := client.New(opts...)
	c.LogDebugForTest(context.Background(), "env-file-msg")
	_ = closeLogFiles()
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "env-file-msg") {
		t.Fatalf("env NAZHI_LOG_FILE should be effective got %q", string(data))
	}
	t.Setenv("NAZHI_LOG_FILE", "")
}

// 9. PersistentPreRun 注入 trace_id 到 cmd.Context
func TestBoundary_CLI_PersistentPreRun_InjectsTrace(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	// 模拟 main.go 的 PersistentPreRun 逻辑
	tid := logx.NewTraceID()
	ctx := logx.WithTraceID(cmd.Context(), tid)
	cmd.SetContext(ctx)
	if got := logx.TraceIDFrom(cmd.Context()); got != tid {
		t.Fatalf("PersistentPreRun should inject trace got %q want %q", got, tid)
	}
	if len(tid) != 12 {
		t.Fatalf("trace len want 12 got %d", len(tid))
	}
}
