// main_panic_recover_test.go 锚定。
// F7/F9 设计契约：cmd/nazhi/main.go 顶层 main() 函数必须有 panic recover 守卫，
// 且 recover 后必须设 pendingExitCode=1，让 main 末尾的
// `if pendingExitCode.Load() != 0 { os.Exit(1) }` 分支触发。
//
// 历史：
//   - F9（旧契约）：main.go 没有 panic recover，cobra Run 回调 panic 时
//     Go runtime 直接打 stack trace + exit code 2，CI 无法区分「用户错误」
//     与「程序 bug」。
//   - F7（当前 finding）：main.go 加了 panic recover 但**没**调 markError()，
//     panic 后 pendingExitCode=0，main 末尾 os.Exit(1) 分支不触发，
//     进程以 exit 0 退出——CI 误判 panic 为成功。
//
// 修复契约
//   - panic 发生后 recover
//   - recover 闭包内调 markError() 设 pendingExitCode=1
//   - 走 printError 路径（与正常 error 路径一致）
//   - debug.Stack() 输出到 stderr 辅助生产问题定位
//   - closeAllClients defer 仍跑（释放 ONNX session + tempDir + keep-alive）
//
// 测试策略：
//   - TestMain_PanicRecover_ASTInspect：AST 静态扫描 main.go 含 defer recover 守卫
//   - TestMain_PanicRecover_ExitCodeOne：AST 静态扫描 recover 闭包内含 markError
//     或 pendingExitCode.Store(1) —— 这是本轮 F7 finding 的核心契约
//   - TestMain_PanicRecover_EndToEnd：在测试进程内复现 panic 场景，验证
//     panic recover 路径会输出 JSON envelope
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

// TestMain_PanicRecover_ASTInspect 静态扫描 main.go 函数体，断言存在
// defer 闭包调用 recover()，且该 defer 在 main 顶层（不是某个子函数里）。
// F9 设计：main() 顶部加
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        pendingExitCode.Store(1)
//	        printError(fmt.Errorf("panic: %v", r))
//	    }
//	}()
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

// TestMain_PanicRecover_ExitCodeOne 锚定 F7 「统一 exit code 1」契约（本轮 finding）。
//
// finding：原 main.go 顶层 defer recover 只把 debug.Stack() 写到 stderr，
// 没调 markError()，panic 后 pendingExitCode 仍为 0，main 函数末尾
// os.Exit(1) 分支不会触发，进程以 exit 0 退出，CI 误判成功。
//
// 为什么用 AST 静态扫描而不是直接调 main()：
//
//	main() 含 os.Exit(1)，go test 子进程无法直接执行
//	go test 进程内调 main() 会因 os.Exit 中断测试运行
//	AST 扫描确保 invariant 在源码层永不被破坏
//
// 验证：main.go 的 panic recover 闭包体里必须含以下其一
//   - markError() 调用
//   - pendingExitCode.Store(<常量>) 调用
func TestMain_PanicRecover_ExitCodeOne(t *testing.T) {
	got := mainPanicRecoverSetsExitCode(t)
	if !got {
		t.Errorf("F7 修复：main 顶层 panic recover 闭包内必须调 markError() 或 pendingExitCode.Store(1)\n" +
			"否则 panic 后 pendingExitCode=0，main 末尾 os.Exit(1) 分支不触发，\n" +
			"进程以 exit 0 退出，CI 脚本误判成功。")
	}
}

// mainPanicRecoverSetsExitCode 用 AST 静态扫描 main.go 的 panic recover
// 闭包体，断言含 markError() 或 pendingExitCode.Store(1) 调用。
// 允许不同实现风格但语义必须一致。
func mainPanicRecoverSetsExitCode(t *testing.T) bool {
	t.Helper()
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

	// 找 defer recover 闭包
	foundSetsExitCode := false
	ast.Inspect(mainFn.Body, func(n ast.Node) bool {
		deferStmt, ok := n.(*ast.DeferStmt)
		if !ok {
			return true
		}
		funcLit, ok := deferStmt.Call.Fun.(*ast.FuncLit)
		if !ok {
			return true
		}
		// 闭包体内必须有 recover() 调用
		hasRecover := false
		ast.Inspect(funcLit.Body, func(n2 ast.Node) bool {
			if call, ok := n2.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "recover" {
					hasRecover = true
					return false
				}
			}
			return true
		})
		if !hasRecover {
			return true
		}
		// 闭包体内必须有 markError() 或 pendingExitCode.Store(N)
		ast.Inspect(funcLit.Body, func(n2 ast.Node) bool {
			call, ok := n2.(*ast.CallExpr)
			if !ok {
				return true
			}
			// match markError()
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "markError" {
				foundSetsExitCode = true
				return false
			}
			// match pendingExitCode.Store(N)
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Store" {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "pendingExitCode" {
					foundSetsExitCode = true
					return false
				}
			}
			return true
		})
		return foundSetsExitCode
	})
	return foundSetsExitCode
}

// TestMain_PanicRecover_EndToEnd 模拟 cobra Run 回调 panic 场景
// 注册一个会 panic 的子命令到 rootCmd，调用 panicCmd.Run 直接 panic
// 验证 panic recover 后行为（pendingExitCode=1 + JSON envelope 输出）。
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
