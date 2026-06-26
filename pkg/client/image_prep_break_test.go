// image_prep_break_test.go 通过 AST 静态扫描锁定 F4 修复：
// image_prep.go 缩放级联循环不能 `continue` 跳过 `current = resized`。
//
// F4 证据：image_prep.go 缩放级联 `for _, scale := range getScaleFactors()`
// 内 `if err != nil { continue }` 跳过 `current = resized`，下一轮用
// 未更新的 current 计算 w/h → 同一尺寸重复 encodeJPEG 必然同样失败 →
// 浪费 1-7 轮 CPU 后才 break 返回 ErrImageTooLarge。
//
// 修复：`continue` → `break` + logDebug（encodeJPEG 内部错误重试无意义）。
//
// 测试策略：AST 扫描，定位 scaleFactors range 循环，递归查找 continue
// 语句（注释里的字面量"continue"不会被 AST 误判）。
package client

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestImagePrep_ScaleCascadeNoContinue AST 扫描 image_prep.go，
// 断言 scaleFactors range 循环内不能出现 continue 语句。
func TestImagePrep_ScaleCascadeNoContinue(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "image_prep.go", nil, 0)
	if err != nil {
		t.Fatalf("parse image_prep.go: %v", err)
	}

	// 1. 找到 prepareImageForUpload 函数
	var prepFn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "prepareImageForUpload" {
			prepFn = fd
			break
		}
	}
	if prepFn == nil {
		t.Fatal("找不到 prepareImageForUpload 函数")
	}

	// 2. 找到 range getScaleFactors() 的 for 循环
	var scaleLoop *ast.RangeStmt
	ast.Inspect(prepFn.Body, func(n ast.Node) bool {
		if rs, ok := n.(*ast.RangeStmt); ok {
			if call, ok := rs.X.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "getScaleFactors" {
					scaleLoop = rs
					return false
				}
			}
		}
		return true
	})
	if scaleLoop == nil {
		t.Fatal("找不到 `for _, scale := range getScaleFactors()` 循环")
	}

	// 3. 扫描循环 body，禁止 continue（Go 1.26 把 ContinueStmt/BreakStmt
	// 统一为 *ast.BranchStmt，靠 Tok 字段 token.CONTINUE / token.BREAK 区分）
	var foundContinue *ast.BranchStmt
	ast.Inspect(scaleLoop.Body, func(n ast.Node) bool {
		if br, ok := n.(*ast.BranchStmt); ok && br.Tok == token.CONTINUE {
			foundContinue = br
			return false
		}
		return true
	})
	if foundContinue != nil {
		t.Errorf("F4 回归：scaleFactors 循环在 %s 出现 continue 语句。"+
			"continue 会跳过 current = resized 赋值，下一轮重试同尺寸同错误，"+
			"浪费 1-7 轮 CPU 后才在 MinImageDimension 边界 break。必须改为 break。",
			fset.Position(foundContinue.Pos()))
	}

	// 4. 验证修复契约：循环 body 含 break 语句
	var foundBreak *ast.BranchStmt
	ast.Inspect(scaleLoop.Body, func(n ast.Node) bool {
		if br, ok := n.(*ast.BranchStmt); ok && br.Tok == token.BREAK {
			foundBreak = br
			return false
		}
		return true
	})
	if foundBreak == nil {
		t.Errorf("F4 修复契约：scaleFactors 循环必须含 break 语句跳出失败轮次")
	}
}

// TestImagePrep_ScaleCascadeHasLogDebug 验证修复契约：
// 缩放级联循环的错误分支必须配 logDebug 调用。
//
// 用字符串子串匹配（仅在错误处理块注释 anchor 范围内），
// 不易触发字面量误判：定位 `if err != nil {` 锚点 + 下一 break 之间的内容。
func TestImagePrep_ScaleCascadeHasLogDebug(t *testing.T) {
	src, err := readSource("image_prep.go")
	if err != nil {
		t.Fatalf("读 image_prep.go: %v", err)
	}
	body := string(src)

	// 锚点：encodeJPEG 调用之后紧随的 `if err != nil {` 错误处理块。
	// 源码用两步式（先 data, err = encodeJPEG(...)，再单独 if err != nil），
	// 不是 if-init 复合形式。
	anchor := "data, err = encodeJPEG(resized, 40)"
	idx := strings.Index(body, anchor)
	if idx < 0 {
		t.Fatalf("找不到 %q 锚点，源码结构可能改了", anchor)
	}
	// 取该 if 块后续 600 字符（足够看到 break + logDebug）
	block := body[idx:]
	if len(block) > 600 {
		block = block[:600]
	}

	if !strings.Contains(block, "break") {
		t.Errorf("F4 修复契约：encodeJPEG 失败分支必须含 break（不能 continue），实际块:\n%s", block)
	}
	if !strings.Contains(block, "logDebug") {
		t.Errorf("F4 修复契约：encodeJPEG 失败 break 前应 logDebug 记录原因，实际块:\n%s", block)
	}
}

// readSource 包内 helper：读当前包目录下的源码文件。
func readSource(name string) ([]byte, error) {
	return osReadFile(name)
}

// osReadFile 用 os.ReadFile 读文件，单独函数便于未来 mock。
func osReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
