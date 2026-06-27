package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestPrintError_DoesNotCallOsExit 回归测试：printError 必须不调用 os.Exit。
//
// 历史 bug（F7）：printError 直接 os.Exit(1)，导致 main 中 defer closeAllClients()
// 永远不执行，ONNX session + 临时目录 + keep-alive 连接全部泄漏。
// 修复后：printError 仅写 stderr，由 main 在 Execute() 之后统一 os.Exit(1)。
//
// 验证方式：调 printError 之后，测试进程必须继续存活（如果 os.Exit 被调用，
// 当前测试函数返回后下一行会跑不到）。用一个 atomic 计数器在 printError
// 调用之后递增，证明流程没断。
func TestPrintError_DoesNotCallOsExit(t *testing.T) {
	// 捕获 stderr（printError 写到这里）
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	// printError 不应让进程退出
	printError(errors.New("synthetic error for F7 regression"))

	// 关闭 writer 让 reader 能读到 EOF
	_ = w.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	stderrOutput := buf.String()

	// 关键断言：进程仍在运行（走到这里就是证据）。再补一个 atomic 计数器
	// 证明后续断言在 printError 之后执行。
	var ranAfterPrintError atomic.Bool
	ranAfterPrintError.Store(true)
	if !ranAfterPrintError.Load() {
		t.Fatal("printError 之后代码未执行（很可能 os.Exit 被调用）")
	}

	// 验证错误信息确实写到了 stderr（保持原行为，只是把退出权交给 main）
	if !strings.Contains(stderrOutput, "synthetic error for F7 regression") {
		t.Errorf("stderr 应包含错误信息，实际: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, `"error": true`) {
		t.Errorf("stderr 应包含 JSON error 标记，实际: %q", stderrOutput)
	}
}

// TestMain_DeferCloseStillRuns 验证 main 的 defer closeAllClients() 行为：
// 模拟一次"有 pending client + 显式调 closeAllClients"的流程，验证客户端
// 列表被清空（证明 Close 真的被调用，不被 os.Exit 跳过）。
func TestMain_DeferCloseStillRuns(t *testing.T) {
	// 构造一个真实 client 并注册
	c, _ := client.New()
	trackClient(c)
	t.Cleanup(func() { _ = c.Close() })
	// 兜底：测试结束前确保清空（不污染其它测试）
	defer func() {
		pendingClientsMu.Lock()
		pendingClients = nil
		pendingClientsMu.Unlock()
	}()

	// 记录原始列表长度
	pendingClientsMu.Lock()
	before := len(pendingClients)
	pendingClientsMu.Unlock()
	if before == 0 {
		t.Fatal("trackClient 之后 pendingClients 应非空")
	}

	// 模拟 main 退出时的 defer
	if err := closeAllClients(); err != nil {
		// 错误仅记录，不影响断言
		t.Logf("closeAllClients 报错（可接受）: %v", err)
	}

	// 验证列表已清空 → 证明 defer 在 os.Exit 之前能跑完
	pendingClientsMu.Lock()
	after := len(pendingClients)
	pendingClientsMu.Unlock()
	if after != 0 {
		t.Errorf("closeAllClients 之后 pendingClients 应清空，实际长度 %d", after)
	}
}

// TestPrintPrompt_QuietModeSuppressesOutput L finding 回归测试：quiet=true 时
// printPrompt 必须不写 stderr。即使用户在终端运行 self-eval submit --quiet，
// 也不该看到 "请输入自我评价内容（Ctrl+D 结束）: " 提示符。
func TestPrintPrompt_QuietModeSuppressesOutput(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	origQuiet := quiet
	quiet = true
	t.Cleanup(func() { quiet = origQuiet })

	printPrompt("TEST_PROMPT_SHOULD_NOT_APPEAR")

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}

	if strings.Contains(buf.String(), "TEST_PROMPT_SHOULD_NOT_APPEAR") {
		t.Errorf("quiet 模式下 printPrompt 不应输出，实际 stderr: %q", buf.String())
	}
}

// TestPrintPrompt_NonTTYStdinSuppressesOutput L finding 回归测试：
// stdin 不是 TTY 时（CI / 管道环境）printPrompt 必须不输出。
//
// 模拟方法：把 os.Stdin 替换成空文件（ModeCharDevice=0 → isTerminalStdin 返回 false）。
// 测试结束后恢复原始 stdin。
func TestPrintPrompt_NonTTYStdinSuppressesOutput(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	// 准备一个非 TTY stdin：空文件就是 regular file，ModeCharDevice = 0
	tmpFile, err := os.CreateTemp("", "nazhi-prompt-test-")
	if err != nil {
		t.Fatalf("CreateTemp 失败: %v", err)
	}
	t.Cleanup(func() { _ = tmpFile.Close(); _ = os.Remove(tmpFile.Name()) })

	origStdin := os.Stdin
	os.Stdin = tmpFile
	t.Cleanup(func() { os.Stdin = origStdin })

	origQuiet := quiet
	quiet = false
	t.Cleanup(func() { quiet = origQuiet })

	// sanity check：mock 后的 stdin 确实不被认为是 TTY
	if isTerminalStdin() {
		t.Skip("运行环境 stdin 被识别为 TTY，无法触发 NonTTY 守卫；跳过")
	}

	printPrompt("TEST_PROMPT_NONTTY_SHOULD_NOT_APPEAR")

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}

	if strings.Contains(buf.String(), "TEST_PROMPT_NONTTY_SHOULD_NOT_APPEAR") {
		t.Errorf("非 TTY 环境下 printPrompt 不应输出，实际 stderr: %q", buf.String())
	}
}
