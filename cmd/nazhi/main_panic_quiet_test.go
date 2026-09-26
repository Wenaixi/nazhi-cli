package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestMain_PanicRecover_QuietGuardsRecoverPanic 锚定 --quiet 静默契约：
// --quiet 承诺「关闭所有 stderr 输出」，但 main.go 顶层 panic recover 闭包
// 此前无条件调 recoverx.RecoverPanic，而 RecoverPanic 内部无条件
// os.Stderr.Write(debug.Stack())，导致 --quiet 时整段 goroutine stack 泄漏。
// printError 的 quiet 守卫已存在（quiet 时不写信封但照设退出码），
// 本测试锁定 stack 写入同样必须受 if !quiet 守卫。
func TestMain_PanicRecover_QuietGuardsRecoverPanic(t *testing.T) {
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

	// 在 main 顶层 panic recover 闭包内查找受 if !quiet 守卫的 RecoverPanic 调用
	guarded := false
	ast.Inspect(mainFn.Body, func(n ast.Node) bool {
		deferStmt, ok := n.(*ast.DeferStmt)
		if !ok {
			return true
		}
		funcLit, ok := deferStmt.Call.Fun.(*ast.FuncLit)
		if !ok {
			return true
		}
		// 闭包内必须有 recover()
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
		// 该闭包内 recoverx.RecoverPanic 调用必须位于 if !quiet 守卫内
		ast.Inspect(funcLit.Body, func(n2 ast.Node) bool {
			ifStmt, ok := n2.(*ast.IfStmt)
			if !ok {
				return true
			}
			// cond 必须是 !quiet
			unary, ok := ifStmt.Cond.(*ast.UnaryExpr)
			if !ok || unary.Op != token.NOT {
				return true
			}
			ident, ok := unary.X.(*ast.Ident)
			if !ok || ident.Name != "quiet" {
				return true
			}
			// if !quiet body 内必须有 RecoverPanic 调用
			ast.Inspect(ifStmt.Body, func(n3 ast.Node) bool {
				call, ok := n3.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "RecoverPanic" {
					guarded = true
					return false
				}
				return true
			})
			return !guarded
		})
		return !guarded
	})

	if !guarded {
		t.Errorf("--quiet 契约：main 顶层 panic recover 闭包内 recoverx.RecoverPanic 调用必须受 if !quiet 守卫，\n" +
			"否则 --quiet 时 RecoverPanic 无条件写 debug.Stack() 到 stderr，击穿「关闭所有 stderr 输出」契约")
	}
}
