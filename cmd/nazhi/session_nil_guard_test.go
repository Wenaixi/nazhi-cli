// session_nil_guard_test.go 锚定 B5 修复契约。
//
// B5 证据：cmd/nazhi/session.go:70 printJSON(info) 直接打印，未检查 info == nil。
// 虽然 GetMyInfo 已通过 ErrEmptyUserInfo 哨兵避免 (nil, nil)，但防 future regression
// 导致 info 为 nil 时输出裸 null。
//
// 修复：在 printJSON(info) 前加 if info == nil 守卫，调 markError + 输出
// status envelope（与 ErrEmptyUserInfo 分支对称）。
//
// 测试策略：AST 静态扫描 session.go 的 sessionActivateCmd.Run 函数体，
// 断言在 printJSON(info) 调用之前必须出现 `info == nil` 守卫。
// 防止 future refactor 删掉这个守卫造成裸 null 回归。
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestSessionActivate_HasNilGuardBeforePrintJSON AST 扫描 sessionActivateCmd.Run 函数
// 断言 printJSON(info) 调用之前必须显式检查 info == nil（防御 future regression）。
func TestSessionActivate_HasNilGuardBeforePrintJSON(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "session.go", nil, 0)
	if err != nil {
		t.Fatalf("parse session.go: %v", err)
	}

	// 1. 找 sessionActivateCmd 变量声明
	var runFunc *ast.FuncLit
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, sp := range gd.Specs {
			vs, ok := sp.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, id := range vs.Names {
				if id.Name != "sessionActivateCmd" || i >= len(vs.Values) {
					continue
				}
				// 兼容 `&cobra.Command{...}` 和 `cobra.Command{...}`
				var cl *ast.CompositeLit
				switch v := vs.Values[i].(type) {
				case *ast.CompositeLit:
					cl = v
				case *ast.UnaryExpr:
					cl, _ = v.X.(*ast.CompositeLit)
				}
				if cl == nil {
					continue
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					k, ok := kv.Key.(*ast.Ident)
					if !ok || k.Name != "Run" {
						continue
					}
					fl, ok := kv.Value.(*ast.FuncLit)
					if !ok {
						continue
					}
					runFunc = fl
				}
			}
		}
	}
	if runFunc == nil {
		t.Fatal("找不到 sessionActivateCmd.Run 函数")
	}

	// 2. 找 printJSON(info) 调用位置
	var printJSONPos token.Pos
	ast.Inspect(runFunc.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "printJSON" {
				if len(call.Args) == 1 {
					if arg, ok := call.Args[0].(*ast.Ident); ok && arg.Name == "info" {
						printJSONPos = call.Pos()
						return false
					}
				}
			}
		}
		return true
	})
	if printJSONPos == 0 {
		t.Fatal("sessionActivateCmd.Run 未发现 printJSON(info) 调用")
	}

	// 3. 找 info == nil 守卫（IfStmt with == nil 检查 on info）
	// 必须在 printJSONPos 之前出现
	var nilGuardPos token.Pos
	ast.Inspect(runFunc.Body, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			// 检查 cond: BinaryExpr with Op == and Left 是 info, Right 是 nil
			if be, ok := ifStmt.Cond.(*ast.BinaryExpr); ok && be.Op == token.EQL {
				if left, ok := be.X.(*ast.Ident); ok && left.Name == "info" {
					if right, ok := be.Y.(*ast.Ident); ok && right.Name == "nil" {
						if ifStmt.Pos() < printJSONPos {
							if nilGuardPos == 0 || ifStmt.Pos() > nilGuardPos {
								nilGuardPos = ifStmt.Pos()
							}
						}
					}
				}
			}
		}
		return true
	})

	if nilGuardPos == 0 {
		printJSONLine := fset.Position(printJSONPos).Line
		t.Errorf("B5 守卫缺失：sessionActivateCmd.Run 在 printJSON(info) (line %d) 之前必须有 `if info == nil` 守卫。\n"+
			"future regression：如果 SDK 回归到返回 (nil, nil)，cmd 层会输出裸 null。",
			printJSONLine)
		return
	}
	t.Logf("✓ B5 修复锚定：info == nil 守卫在 line %d，printJSON(info) 在 line %d",
		fset.Position(nilGuardPos).Line, fset.Position(printJSONPos).Line)
}
