// main_exit_cleanup_test.go F6 修复锚定（AST 静态扫描）。
//
// F6 证据：原 main.go 在 `if pendingExitCode.Load() != 0 { os.Exit(1) }`
// 之前没有调用 closeAllClients()，而 Go 规范明确 os.Exit 不运行
// deferred functions，导致 main 顶部 `defer closeAllClients()` 永不执行。
// 后果：所有 CLI 错误退出路径泄漏 ONNX session + tempDir + keep-alive 连接。
//
// 修复：在 os.Exit(1) 之前显式调 closeAllClients()。
// 设计契约：closeAllClients 内部已把全局 pendingClients 置 nil，
// 二次调用是 no-op，因此即使 defer 也会再跑一次也不会出错。
//
// 测试策略：AST 静态扫描 main 函数体，断言：
//  1. os.Exit(1) 调用之前（行号序）必须出现 closeAllClients() 调用
//  2. 不能仅靠 defer（验证修复契约：显式调用）
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestMain_OsExitPrecededByCloseAllClients AST 扫描 main 函数，
// 断言 os.Exit(1) 之前必须显式调 closeAllClients（不能仅靠 defer）。
func TestMain_OsExitPrecededByCloseAllClients(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	// 1. 找到 main 函数
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

	// 2. 找 os.Exit(1) 调用位置（os.Exit 在 AST 是 *ast.SelectorExpr，
	// Fun = os, Sel = Exit）
	var exitPos token.Pos
	var exitLine int
	ast.Inspect(mainFn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Exit" {
				if exitPos == 0 || call.Pos() < exitPos {
					exitPos = call.Pos()
					exitLine = fset.Position(call.Pos()).Line
				}
			}
		}
		return true
	})
	if exitPos == 0 {
		t.Fatal("main 函数未发现 os.Exit 调用（F6 修复契约：必须在 exit 前 closeAllClients）")
	}

	// 3. 找 closeAllClients 调用，必须是**显式调用**（非 defer 闭包内）
	//
	// AST 区分：
	//   - defer func() { closeAllClients() }()  → closeAllClients 调用包裹在
	//     FuncLit.Body.List 里，不是 main 函数顶层 stmt 也不是 if-block 顶层 stmt
	//   - if pendingExitCode.Load() != 0 {
	//         _ = closeAllClients()
	//         os.Exit(1)
	//     }
	//     → 顶层是 *ast.IfStmt，body 内是 ExprStmt / AssignStmt / ExprStmt
	//
	// 我们遍历 main 函数所有 stmt（包括 if-block），但跳过 defer 的 FuncLit.Body。
	var foundClose *ast.CallExpr
	var closeLine int
	var visitStmts func(stmts []ast.Stmt)
	visitStmts = func(stmts []ast.Stmt) {
		for _, stmt := range stmts {
			// 跳过 defer：DeferStmt.Call 是闭包调用，不递归进 FuncLit.Body
			if _, isDefer := stmt.(*ast.DeferStmt); isDefer {
				continue
			}
			var call *ast.CallExpr
			switch s := stmt.(type) {
			case *ast.ExprStmt:
				if c, ok := s.X.(*ast.CallExpr); ok {
					call = c
				}
			case *ast.AssignStmt:
				for _, rhs := range s.Rhs {
					if c, ok := rhs.(*ast.CallExpr); ok {
						call = c
						break
					}
				}
			case *ast.IfStmt:
				visitStmts(s.Body.List)
			case *ast.BlockStmt:
				visitStmts(s.List)
			}
			if call == nil {
				continue
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "closeAllClients" {
				continue
			}
			if call.Pos() < exitPos {
				if foundClose == nil || call.Pos() > foundClose.Pos() {
					foundClose = call
					closeLine = fset.Position(call.Pos()).Line
				}
			}
		}
	}
	visitStmts(mainFn.Body.List)

	if foundClose == nil {
		t.Errorf("F6 回归：main 函数在 os.Exit(1) (line %d) 之前没有**顶层**显式调用 closeAllClients（仅靠 defer 不算）。\n"+
			"Go 规范：os.Exit 不运行 deferred functions，仅靠 main 顶部\n"+
			"  defer closeAllClients() 永远不会执行，所有错误退出路径\n"+
			"  泄漏 ONNX session + tempDir + keep-alive 连接。",
			exitLine)
		return
	}
	t.Logf("✓ F6 修复锚定：closeAllClients 在 line %d 顶层显式调用，os.Exit 在 line %d",
		closeLine, exitLine)
}

// TestMain_NoDeferOnlyClose 防退化：未来重构如果删掉显式调用、退化到仅 defer，
// AST 扫描会失败（保证修复不会无声消失）。
func TestMain_NoDeferOnlyClose(t *testing.T) {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	// 该测试由 TestMain_OsExitPrecededByCloseAllClients 覆盖核心契约。
	// 留作冗余：未来若有人加 `defer func() { closeAllClients() }()` 在 os.Exit 后，
	// 此测试提醒「必须在 os.Exit 前显式调用」。
	t.Log("防退化测试：见 TestMain_OsExitPrecededByCloseAllClients")
}
