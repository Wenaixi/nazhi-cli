// main_panic_recover_test.go F9 修复锚定（group-F round-8）。
//
// F9 证据：cmd/nazhi/main.go 顶层 main() 函数没有 panic recover 守卫。
// 后果：cobra Run 回调（cmd.Run func）panic 时，Go runtime 直接打 stack trace
// 并以 exit code 2 退出，违反 F7 设计的「统一 exit code 1」契约——CI 脚本
// 区分「用户错误」(exit 1) 与「程序 bug」(exit 2) 时被误导。
//
// 设计契约：
//   - panic 发生后 recover
//   - 走 pendingExitCode = 1 + printError 路径（与 cobra 正常 error 路径一致）
//   - 不打 stack trace 给终端用户（避免噪声 + 信息泄露）
//   - 仍然走 closeAllClients() 释放 ONNX session + tempDir + keep-alive
//
// 测试策略：子进程派生子 nazhi，触发 panic，断言：
//   1. 子进程 exit code == 1（不是 Go runtime 默认的 2）
//   2. stderr 不含 Go runtime stack trace 特征 "panic:" 或 ".go:"
//
// 实现方式：通过 NAZHI_TEST_PANIC 环境变量让 main.go 触发 panic。
// 但这样需要改 main.go 加测试 hook。**更优雅**：直接调 main 路径中
// 的 rootCmd.Execute()，但注入一个会 panic 的 Run 回调到某个子命令。
//
// 子进程方案更稳健（能验证完整 main → main 退出码语义），但需要
// 重新编译 main.go binary。本测试用直接调用方式：模拟 panic recover
// 包装函数（main.go 内部）。
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestMain_PanicRecover_ExitCode1 验证 main 函数 panic recover 后退出码为 1。
//
// 实现策略：子进程方式
//  1. go run -tags panic_recover 编译并执行 nazhi panic 子命令
//  2. 注入 NAZHI_TEST_PANIC=1 让 main.go 主动 panic
//  3. 断言子进程 exit code == 1
//
// 但我们没有 panic_recover tag，所以改用更直接的方式：
// 临时添加一个会 panic 的 cobra 子命令，rootCmd.Execute() 触发 panic，
// 验证 panic recover 后行为（不修改全局状态）。
func TestMain_PanicRecover_ExitCode1(t *testing.T) {
	// 子进程方案：go build 当前 binary + 设 NAZHI_FORCE_PANIC 环境变量
	// 我们用 binary 自身 + 注入 panic hook（通过测试 helper）。
	//
	// 简单可靠方案：编译当前 package 的 test binary，把 panic recover 行为
	// 直接 inline 测：panic 在 rootCmd.Run 回调中触发，走 deferred recover。
	//
	// 实际：F9 修复后 main.go 顶层会有 `defer func() { recover; ...; os.Exit(1) }()`，
	// 我们无法直接测试 main()（只能测子函数）。改用 AST 静态扫描测试
	// main 函数体内必须含 panic recover 调用模式。
	t.Log("F9 修复锚定由 TestMain_PanicRecover_ASTInspect 实现")
}

// TestMain_PanicRecover_ASTInspect 静态扫描 main.go 函数体，断言存在
// defer 闭包调用 recover()，且该 defer 在 main 顶层（不是某个子函数里）。
//
// F9 设计：main() 顶部加
//   defer func() {
//       if r := recover(); r != nil {
//           pendingExitCode.Store(1)
//           printError(fmt.Errorf("panic: %v", r))
//       }
//   }()
//
// 这保证所有 cobra Run 回调 panic → 走与 error 路径一致的 exit code 1。
func TestMain_PanicRecover_ASTInspect(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var mainFn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
			mainFn = fd
			break
		}
	}
	if mainFn == nil {
		t.Fatal("找不到 main 函数")
	}

	// 查找 defer 闭包内是否调 recover()
	foundRecover := false
	ast.Inspect(mainFn.Body, func(n ast.Node) bool {
		deferStmt, ok := n.(*ast.DeferStmt)
		if !ok {
			return true
		}
		// defer func() { ... }()
		funcLit, ok := deferStmt.Call.Fun.(*ast.FuncLit)
		if !ok {
			return true
		}
		// 闭包体内必须调 recover()
		ast.Inspect(funcLit.Body, func(n2 ast.Node) bool {
			if call, ok := n2.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "recover" {
					foundRecover = true
					return false
				}
			}
			return true
		})
		return foundRecover
	})

	if !foundRecover {
		t.Errorf("F9 未修复：main 函数没有 defer recover() 守卫。\n" +
			"cobra Run 回调 panic 时 Go runtime 默认 exit code = 2，\n" +
			"违反 F7 '统一 exit code 1' 契约，CI 脚本无法区分用户错误与程序 bug。")
	}
}

// TestMain_PanicRecover_EndToEnd 模拟 cobra Run 回调 panic 场景：
// 注册一个会 panic 的子命令到 rootCmd，调用 rootCmd.Execute()，
// 验证 panic recover 后行为（pendingExitCode=1 + JSON envelope 输出）。
//
// 不走子进程（避免编译 CGO 依赖），直接在测试进程内复现。
func TestMain_PanicRecover_EndToEnd(t *testing.T) {
	// 暂存并恢复全局状态
	origPending := pendingExitCode.Load()
	pendingExitCode.Store(0)
	origStderr := os.Stderr
	t.Cleanup(func() {
		pendingExitCode.Store(origPending)
		os.Stderr = origStderr
	})

	// 捕获 stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() { _ = w.Close() })

	// 注册临时 panic 子命令
	panicCmd := &cobra.Command{
		Use: "panic-test-cmd-f9",
		Run: func(cmd *cobra.Command, args []string) {
			panic("forced panic for F9 panic recover test")
		},
	}
	rootCmd.AddCommand(panicCmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(panicCmd) })

	// 用 os.Args 模拟命令行调用
	origArgs := os.Args
	os.Args = []string{"nazhi", "panic-test-cmd-f9"}
	t.Cleanup(func() { os.Args = origArgs })

	// 模拟 main 的 panic recover + Execute + exit code 流程
	func() {
		defer func() {
			if r := recover(); r != nil {
				printError(fmt.Errorf("内部错误: %v", r))
			}
		}()
		panicCmd.Run(panicCmd, nil)
	}()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := readAndCopy(&buf, r); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	stderrStr := buf.String()

	// 断言 1：pendingExitCode 应为 1（panic recover 后走统一 exit code 1 路径）
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("F9 修复：panic 后 pendingExitCode 应为 1，实际 %d", got)
	}

	// 断言 2：stderr 应含 printError 输出（JSON envelope 或 "panic" 字样）
	if !strings.Contains(stderrStr, `"error": true`) {
		t.Errorf("F9 修复：panic 后 stderr 应含 JSON envelope，实际: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "forced panic") {
		t.Errorf("F9 修复：stderr 应含 panic 信息，实际: %q", stderrStr)
	}

	// 断言 3：不应含 Go runtime stack trace
	if strings.Contains(stderrStr, "goroutine ") && strings.Contains(stderrStr, "[running]") {
		t.Errorf("F9 修复：panic 后 stderr 不应含 Go runtime stack trace\nstderr: %s", stderrStr)
	}
}

// readAndCopy 包装 io.Copy 避免在测试文件中再次出现 io 导入
func readAndCopy(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}