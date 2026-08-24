// session_nil_guard_test.go 锚定 nil 守卫契约。
//
// 历史 bug：cmd/nazhi/session.go 原 printJSON(info) 直接打印，未检查 info == nil。
// 虽然 GetMyInfo 已通过 ErrEmptyUserInfo 哨兵避免 (nil, nil)，但防 future regression
// 导致 info 为 nil 时输出裸 null。
//
// 修复：在 printEnvelope(envelope.Success(info)) 前加 if info == nil 守卫。
// 测试策略：AST 静态扫描 session.go 的 sessionActivateCmd.Run 函数体，
// 断言在 printEnvelope 调用之前必须出现 info == nil 或 len(raw) == 0 守卫。
// 防止 future refactor 删掉这个守卫造成裸 null 回归。
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestSessionActivate_HasNilGuardBeforePrintEnvelope AST 扫描 sessionActivateCmd.Run 函数
// 断言 printEnvelope(envelope.Success(...)) 调用之前必须显式检查 info == nil 或
// len(raw) == 0（防御 future regression）。
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

	// 2. 找 printEnvelope(envelope.Success(...)) 调用位置
	// 匹配 envelope.Success(info) 或 envelope.Success(json.RawMessage(raw)) 等
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
		printEnvPos = call.Pos()
		return false
	})
	if printEnvPos == 0 {
		t.Fatal("sessionActivateCmd.Run 未发现 printEnvelope(envelope.Success(...)) 调用")
	}

	// 3. 找守卫（IfStmt with == nil 或 len() == 0 检查）
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
		// 兼容：info == nil 或 len(raw) == 0
		if matched := isNilOrLengthGuard(be); matched {
			if ifStmt.Pos() < printEnvPos {
				if nilGuardPos == 0 || ifStmt.Pos() > nilGuardPos {
					nilGuardPos = ifStmt.Pos()
				}
			}
		}
		return true
	})

	if nilGuardPos == 0 {
		printEnvLine := fset.Position(printEnvPos).Line
		t.Errorf("守卫缺失：sessionActivateCmd.Run 在 printEnvelope(envelope.Success(...)) (line %d) 之前必须有 nil/空守卫。\n"+
			"future regression：如果 SDK 回归到返回 (nil, nil)，cmd 层会输出裸 null。",
			printEnvLine)
		return
	}
	t.Logf("✓ 修复锚定：nil/空守卫在 line %d，printEnvelope 在 line %d",
		fset.Position(nilGuardPos).Line, fset.Position(printEnvPos).Line)
}

// isNilOrLengthGuard 判断 BinaryExpr 是否为 info == nil 或 len(x) == 0 守卫。
func isNilOrLengthGuard(be *ast.BinaryExpr) bool {
	// info == nil
	if ident, ok := be.X.(*ast.Ident); ok {
		if y, ok := be.Y.(*ast.Ident); ok && ident.Name == "info" && y.Name == "nil" {
			return true
		}
	}
	if be.Y == nil {
		return false
	}
	// nil == info
	if ident, ok := be.Y.(*ast.Ident); ok {
		if x, ok := be.X.(*ast.Ident); ok && ident.Name == "info" && x.Name == "nil" {
			return true
		}
	}
	// len(raw) == 0
	if call, ok := be.X.(*ast.CallExpr); ok {
		if f, ok := call.Fun.(*ast.Ident); ok && f.Name == "len" && len(call.Args) == 1 {
			if lit, ok := be.Y.(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "0" {
				return true
			}
		}
	}
	return false
}
