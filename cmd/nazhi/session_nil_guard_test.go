// session_nil_guard_test.go 锚定 B5 修复契约。
//
// B5 证据：cmd/nazhi/session.go 原 printJSON(info) 直接打印，未检查 info == nil。
// 虽然 GetMyInfo 已通过 ErrEmptyUserInfo 哨兵避免 (nil, nil)，但防 future regression
// 导致 info 为 nil 时输出裸 null。
//
// 修复：在 printEnvelope(envelope.Success(info)) 前加 if info == nil 守卫。
// 测试策略：AST 静态扫描 session.go 的 sessionActivateCmd.Run 函数体，
// 断言在 printEnvelope 调用之前必须出现 `info == nil` 守卫。
// 防止 future refactor 删掉这个守卫造成裸 null 回归。
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestSessionActivate_HasNilGuardBeforePrintEnvelope AST 扫描 sessionActivateCmd.Run 函数
// 断言 printEnvelope(envelope.Success(info)) 调用之前必须显式检查 info == nil
// （防御 future regression）。
func TestSessionActivate_HasNilGuardBeforePrintEnvelope(t *testing.T) {
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

	// 2. 找 printEnvelope(envelope.Success(info)) 调用位置
	var printEnvPos token.Pos
	ast.Inspect(runFunc.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "printEnvelope" {
			return true
		}
		// 必须是 envelope.Success(info) 形式
		if len(call.Args) != 1 {
			return true
		}
		inner, ok := call.Args[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Success" {
			return true
		}
		// envelope.Success 的唯一参数必须是 info
		if len(inner.Args) != 1 {
			return true
		}
		if arg, ok := inner.Args[0].(*ast.Ident); ok && arg.Name == "info" {
			printEnvPos = call.Pos()
			return false
		}
		return true
	})
	if printEnvPos == 0 {
		t.Fatal("sessionActivateCmd.Run 未发现 printEnvelope(envelope.Success(info)) 调用")
	}

	// 3. 找 info == nil 守卫（IfStmt with == nil 检查 on info）
	// 必须在 printEnvPos 之前出现
	var nilGuardPos token.Pos
	ast.Inspect(runFunc.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		be, ok := ifStmt.Cond.(*ast.BinaryExpr)
		if !ok || be.Op != token.EQL {
			return true
		}
		left, ok := be.X.(*ast.Ident)
		if !ok || left.Name != "info" {
			return true
		}
		right, ok := be.Y.(*ast.Ident)
		if !ok || right.Name != "nil" {
			return true
		}
		if ifStmt.Pos() < printEnvPos {
			if nilGuardPos == 0 || ifStmt.Pos() > nilGuardPos {
				nilGuardPos = ifStmt.Pos()
			}
		}
		return true
	})

	if nilGuardPos == 0 {
		printEnvLine := fset.Position(printEnvPos).Line
		t.Errorf("B5 守卫缺失：sessionActivateCmd.Run 在 printEnvelope(envelope.Success(info)) (line %d) 之前必须有 `if info == nil` 守卫。\n"+
			"future regression：如果 SDK 回归到返回 (nil, nil)，cmd 层会输出裸 null。",
			printEnvLine)
		return
	}
	t.Logf("✓ B5 修复锚定：info == nil 守卫在 line %d，printEnvelope 在 line %d",
		fset.Position(nilGuardPos).Line, fset.Position(printEnvPos).Line)
}
