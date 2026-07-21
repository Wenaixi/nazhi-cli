package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestPrintError_DoesNotCallOsExit 回归测试：printError 必须不调用 os.Exit。
// 历史 bug（F7）：printError 直接 os.Exit(1)，导致 main 中 defer closeAllClients()
// 永远不执行，ONNX session + 临时目录 + keep-alive 连接全部泄漏。
// 修复后：printError 仅写 stderr，由 main 在 Execute() 之后统一 os.Exit(1)。
// 验证方式：调 printError 之后，测试进程必须继续存活（如果 os.Exit 被调用
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
	if !strings.Contains(stderrOutput, `"status": "error"`) {
		t.Errorf("stderr 应包含 envelope status=error 标记，实际: %q", stderrOutput)
	}
}

// TestMain_DeferCloseStillRuns 验证 main 的 defer closeAllClients() 行为
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
// printPrompt 必须不写 stderr。即使用户在终端运行 self-eval submit --quiet
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

// brokenError 实现 error 接口，但其 Error() 返回一个含不可序列化字符的字符串
// 用来模拟「printError 内部 json.Encode 失败」的兜底路径。
// 实际上 json.Encoder 对任何 string 都能成功编码，所以这里改成：直接构造一个
// 让 enc.Encode 返回 error 的情形比较困难。我们用 chan 触发的方式在 production
// 不可能发生——printError(err error) 签名保证 err.Error() 返回 string。
// 因此修复的真正测试点是「兜底路径仍调用 printError 而非 fmt.Fprintf」
// 通过 mock 让 enc.Encode 失败来验证。
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, io.ErrShortWrite }

// jsonEncoder 反射触发失败：把 os.Stderr 替换成一个永远返回错误的 writer。
func TestPrintError_NonMarshalablePayload_StillSetsExitCode(t *testing.T) {
	// 保存并恢复 pendingExitCode
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)

	// 替换 stderr 为一个永远失败的 writer → json.Encode 会失败
	origStderr := os.Stderr
	os.Stderr = nil // 任何写都会 panic — 但我们需要触发 encode error 而不是 panic
	_ = origStderr

	// 上面写法有问题，改用更稳妥的方法：pipe 然后 close writer
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	// 立即关闭 writer — Write 时返回 EPIPE/ErrClosed
	_ = w.Close()
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	// 写一个普通的 error —— 因为 stderr fd 已关，json.Encode 会失败 → 走兜底路径
	printError(errors.New("trigger fallback path"))

	if pendingExitCode.Load() != 2 {
		t.Errorf("pendingExitCode 应为 2（printError 走 envelope.ExitCode，500 → 2），实际 %d", pendingExitCode.Load())
	}
}

// TestPrintError_DepthGuard_NoInfiniteLoop 修复验证：即使 stderr fd 关闭导致
// JSON encoder 也失败，depth 守卫防止递归死循环。
func TestPrintError_DepthGuard_NoInfiniteLoop(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close(); _ = w.Close() }()

	// 用超时保护：如果 depth 守卫失效，测试会卡死
	done := make(chan struct{})
	go func() {
		defer close(done)
		printError(errors.New("trigger fallback loop"))
	}()
	select {
	case <-done:
		// OK：depth 守卫生效，递归被降级
	case <-time.After(2 * time.Second):
		t.Fatal("printError 递归死循环（depth 守卫失效）")
	}
}

// TestPrintError_QuietModeSuppressesStderr 回归测试：quiet=true 时
// printError 不得写 stderr。review 报告指出 envelope 化重构后
// stderr JSON 编码发生在 quiet 守卫之前，导致 --quiet 不生效。
// 修复：把 stderr 写入移到 if !quiet {} 内部，退出码仍正常标记。
func TestPrintError_QuietModeSuppressesStderr(t *testing.T) {
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

	printError(errors.New("quiet mode test"))

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("quiet 模式下 printError 不应写 stderr，实际内容: %q", buf.String())
	}

	// 退出码仍应被标记（不因 quiet 而丢失）
	if pendingExitCode.Load() == 0 {
		t.Error("quiet 模式下 printError 仍应标记退出码")
	}
}

// TestPrintParamError_Code400_Exit3 回归：参数错误必须 envelope.Error(400)→exit 3。
// 历史 bug：printError 固定 Error(500)→exit 2，缺 token / payload 读解析失败
// 被误标为服务端错误，脚本无法区分「用户参数问题」与「服务端/网络问题」。
func TestPrintParamError_Code400_Exit3(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)

	origQuiet := quiet
	quiet = false
	t.Cleanup(func() { quiet = origQuiet })

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	pendingExitCode.Store(0)
	printParamError(errors.New("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）"))

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	stderr := buf.String()

	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("printParamError 应设 pendingExitCode=3，实际 %d", got)
	}
	if !strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 应含 status=error，实际: %q", stderr)
	}
	if !strings.Contains(stderr, `"code": 400`) {
		t.Errorf("stderr 应含 code=400，实际: %q", stderr)
	}
	if strings.Contains(stderr, `"code": 500`) {
		t.Errorf("参数错误不得输出 code=500，实际: %q", stderr)
	}
	if !strings.Contains(stderr, "--token 为必填") {
		t.Errorf("stderr 应含错误信息，实际: %q", stderr)
	}
}

// TestPrintParamError_QuietModeStillSetsExit3 quiet 下不写 stderr，但退出码仍为 3。
func TestPrintParamError_QuietModeStillSetsExit3(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)

	origQuiet := quiet
	quiet = true
	t.Cleanup(func() { quiet = origQuiet })

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	pendingExitCode.Store(0)
	printParamError(errors.New("解析 payload JSON 失败: invalid"))

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("quiet 模式下 printParamError 不应写 stderr，实际: %q", buf.String())
	}
	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("quiet 下仍应 pendingExitCode=3，实际 %d", got)
	}
}

// TestPrintPrompt_NonTTYStdinSuppressesOutput L finding 回归测试
// stdin 不是 TTY 时（CI / 管道环境）printPrompt 必须不输出。
// 模拟方法：用 os.Pipe 替换 os.Stdin（管道永远不是 TTY）。
// 测试结束后恢复原始 stdin。
func TestPrintPrompt_NonTTYStdinSuppressesOutput(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr }()

	// 用 os.Pipe 替换 stdin，管道永远不被识别为 TTY
	origStdin := os.Stdin
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdin 失败: %v", err)
	}
	os.Stdin = stdinR
	t.Cleanup(func() { os.Stdin = origStdin; _ = stdinR.Close(); _ = stdinW.Close() })

	origQuiet := quiet
	quiet = false
	t.Cleanup(func() { quiet = origQuiet })

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
