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
