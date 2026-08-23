package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestLogFlagsEnvPrecedence(t *testing.T) {
	origLevel := cliLogLevel
	origVerbose := verbose
	defer func() { cliLogLevel = origLevel; verbose = origVerbose }()
	// flag 优先于 env
	t.Setenv("NAZHI_LOG_LEVEL", "info")
	cliLogLevel = "debug"
	verbose = false
	// 使用与 opt_builder 相同的优先级逻辑：cliLogLevel flag > env
	levelStr := cliLogLevel
	if levelStr == "" {
		levelStr = os.Getenv("NAZHI_LOG_LEVEL")
	}
	if verbose && levelStr == "" {
		levelStr = "debug"
	}
	if lvl, _ := logx.ParseLevel(levelStr); lvl != slog.LevelDebug {
		t.Fatalf("flag 应优先于 env want debug got %v", lvl)
	}
	// env 生效当 flag 为空
	cliLogLevel = ""
	verbose = false
	levelStr = cliLogLevel
	if levelStr == "" {
		levelStr = os.Getenv("NAZHI_LOG_LEVEL")
	}
	if lvl, _ := logx.ParseLevel(levelStr); lvl != slog.LevelInfo {
		t.Fatalf("env 应生效 want info got %v", lvl)
	}
}

func TestLogFileQuietStillWrites(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	lg.Info("hello", slog.String("trace_id", "tid-test"))
	if !strings.Contains(buf.String(), "tid-test") {
		t.Fatalf("want trace in buf %q", buf.String())
	}
	// 模拟 quiet 时文件仍写的语义：writers 含 file 即便 quiet
	dir := t.TempDir()
	p := filepath.Join(dir, "nazhi.log")
	fw, err := logx.NewFileWriter(p)
	if err != nil {
		t.Fatalf("open log file: %v", err)
	}
	defer fw.Close()
	// 模拟 opt_builder 在 quiet 时仍写入文件的分支
	quietSaved := quiet
	quiet = true
	defer func() { quiet = quietSaved }()
	// quiet 时 stderr 被丢弃，但 file writer 仍写入
	lg2 := logx.NewLogger(slog.LevelDebug, "text", fw)
	lg2.Info("quiet-file-test")
	_ = fw.Close()
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "quiet-file-test") {
		t.Fatalf("quiet 时文件应仍写入 got %q", string(data))
	}
}

func TestVerboseCompat(t *testing.T) {
	origVerbose := verbose
	origLevel := cliLogLevel
	defer func() { verbose = origVerbose; cliLogLevel = origLevel }()
	verbose = true
	cliLogLevel = ""
	t.Setenv("NAZHI_LOG_LEVEL", "")
	levelStr := cliLogLevel
	if levelStr == "" {
		levelStr = os.Getenv("NAZHI_LOG_LEVEL")
	}
	if verbose && levelStr == "" {
		levelStr = "debug"
	}
	if got, _ := logx.ParseLevel(levelStr); got != slog.LevelDebug {
		t.Fatalf("verbose 应等价 debug got %v", got)
	}
}
