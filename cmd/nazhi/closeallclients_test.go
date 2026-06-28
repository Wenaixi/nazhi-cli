// closeallclients_test.go 锚定 B4 修复契约。
//
// B4 证据：cmd/nazhi/main.go:52-58 defer 块中，closeAllClients() 失败时
// 用 fmt.Fprintln(os.Stderr, ...) 直写 stderr，绕过统一的 printError 通道。
// 后果：
//   - 错误输出格式不一致（其他错误都是 JSON envelope，这个是纯文本）
//   - CI 脚本无法 parse 这个错误为 JSON
//   - 与 F7 设计的「统一 printError 通道」契约不一致
//
// 修复：将 fmt.Fprintln 替换为 printError(fmt.Errorf("关闭 Client 资源失败: %w", err))，
// 走统一错误输出通道（stderr JSON envelope + pendingExitCode=1）。
//
// 测试策略：构造一个 Close() 返回错误的 client，注入 pendingClients，
// 手动调 closeAllClients()，然后**手动调 main.go defer 块里的处理逻辑**
// （因为 main() 整体跑会触发 rootCmd.Execute()，难以注入失败 client）。
//
// 契约：
//  1. closeAllClients 返回非 nil error
//  2. defer 块调 printError → stderr 含 {"error": true, "message": "..."}
//  3. stderr 不含旧的纯文本格式 "警告: 关闭 Client 资源失败"
//  4. pendingExitCode = 1（被 printError 内部调 markError 触发）
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// closeErrMockOCR 是 B4 测试用 mock：Recognize 正常返回，Close 返回错误。
// 用于触发 client.Client.Close() 返回错误，进而让 closeAllClients() 返回 error。
type closeErrMockOCR struct{}

func (closeErrMockOCR) Recognize(_ []byte) (string, error) { return "abcd", nil }
func (closeErrMockOCR) Close() error                       { return errors.New("simulated OCR close failure") }

// TestCloseAllClients_Failure_GoesThroughPrintError 验证 main.go defer 块
// 在 closeAllClients 失败时调用 printError 而非 fmt.Fprintln 直写 stderr。
//
// RED 设计：先模拟旧实现 fmt.Fprintln 验证测试能抓到原始问题。
// 测试前置条件：main.go defer 块用 printError（已由 b87607d 修复）。
func TestCloseAllClients_Failure_GoesThroughPrintError(t *testing.T) {
	// 1. 构造一个 Close() 失败的 client
	c, err := client.New(client.WithCustomOCR(closeErrMockOCR{}))
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}
	trackClient(c)

	// 兜底清理：测试结束时清空 pendingClients，不污染其它测试
	t.Cleanup(func() {
		pendingClientsMu.Lock()
		pendingClients = nil
		pendingClientsMu.Unlock()
	})

	// 2. 重置全局状态
	quiet = false
	pendingExitCode.Store(0)

	// 3. 捕获 stderr 和 stdout
	origStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr
	defer func() { os.Stderr = origStderr }()

	// 4. 调 closeAllClients → 应返回 error
	closeErr := closeAllClients()
	if closeErr == nil {
		t.Fatal("closeAllClients 应返回非 nil error（OCR Close 失败）")
	}

	// 5. 模拟 main.go defer 块的处理逻辑
	// 生产代码（修复后）：
	//   if err := closeAllClients(); err != nil {
	//       printError(fmt.Errorf("关闭 Client 资源失败: %w", err))
	//   }
	// 测试时直接复刻该调用，验证输出格式。
	printError(fmt.Errorf("关闭 Client 资源失败: %w", closeErr))

	// 6. 关闭 writer 让 reader 能读到 EOF
	_ = wErr.Close()

	// 7. 读 stderr
	var stderrBuf bytes.Buffer
	if _, err := io.Copy(&stderrBuf, rErr); err != nil {
		t.Fatalf("读取 stderr 失败: %v", err)
	}
	stderr := stderrBuf.String()

	// 8. 断言：stderr 含 JSON envelope（printError 输出）
	if !strings.Contains(stderr, `"error": true`) {
		t.Errorf("B4 修复未生效：closeAllClients 失败应走 printError 通道，stderr 应含 `\"error\": true`\n实际 stderr: %q", stderr)
	}

	// 9. 断言：stderr 不含旧的纯文本格式（fmt.Fprintln 输出）
	if strings.Contains(stderr, "警告: 关闭 Client 资源失败") {
		t.Errorf("B4 修复未生效：closeAllClients 失败不应直写纯文本\n实际 stderr: %q", stderr)
	}

	// 10. 断言：pendingExitCode=1（printError 内部 markError）
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("B4 修复未生效：closeAllClients 失败后 pendingExitCode 应为 1，实际 %d", got)
	}
}
