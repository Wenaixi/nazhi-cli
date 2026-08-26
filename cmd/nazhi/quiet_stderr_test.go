package main

import (
	"os"
	"testing"
)

// TestQuiet_SuppressesConfigWarnings 锁定 --quiet 的 stderr 静默契约：
// 帮助文案承诺「关闭所有 stderr 输出」，配置回落类告警（timeout/log-level/log-format）
// 必须经 warnToStderr 收口——quiet 时丢弃，非 quiet 时照常输出。
func TestQuiet_SuppressesConfigWarnings(t *testing.T) {
	origQuiet := quiet
	quiet = true
	defer func() { quiet = origQuiet }()

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	warnToStderr("warn: timeout 值 -1 无效（flag 或环境变量），使用默认 15 秒超时\n")

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n > 0 {
		t.Errorf("--quiet 下配置告警仍直写 stderr: %q", string(buf[:n]))
	}
}

// TestNoQuiet_ConfigWarningsStillEmitted 非 quiet 路径保持原有告警行为。
func TestNoQuiet_ConfigWarningsStillEmitted(t *testing.T) {
	origQuiet := quiet
	quiet = false
	defer func() { quiet = origQuiet }()

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	warnToStderr("warn: 配置告警探针\n")

	_ = w.Close()
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if n == 0 {
		t.Fatal("非 quiet 下配置告警应照常输出到 stderr")
	}
}
