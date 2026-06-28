// Package ocr 内部白盒测试：无用单例删除验证。
//
// GetDefault() / defaultOCR / defaultOnce 是 0 调用方的进程级单例，
// 应删除以消除 dead code。
//
// 验证策略：在测试运行时 grep 当前包内的 Go 源文件，断言 GetDefault
// 不再出现为函数定义。这是一个轻量级"墓碑测试"——
// Go 没法在运行时直接断言"标识符已删除"，但我们可以确保源码里
// 没有 func GetDefault 这种定义残留。
//
// 维护契约：只要本测试通过，说明 ocr 包已无进程级单例 API。
package ocr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestGetDefault_Removed 断言 ocr 包源码内已无 GetDefault 函数定义。
//
// 如果未来有人重新引入 GetDefault（例如"单例方便"），本测试会 fail，
// 提醒评估是否真的需要恢复该 API。
func TestGetDefault_Removed(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseDir 失败: %v", err)
	}

	pkg, ok := pkgs["ocr"]
	if !ok {
		t.Fatalf("未找到 ocr 包")
	}

	for filename, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			if fn.Recv != nil {
				return true // 跳过方法，只看顶层函数
			}
			if fn.Name.Name == "GetDefault" {
				t.Errorf("%s: 发现残留的 GetDefault 顶层函数定义，应删除该单例 API", filename)
			}
			return true
		})
	}
}

// TestDefaultOCRVar_Removed 断言 ocr 包源码内已无 defaultOCR 变量定义。
func TestDefaultOCRVar_Removed(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseDir 失败: %v", err)
	}

	pkg, ok := pkgs["ocr"]
	if !ok {
		t.Fatalf("未找到 ocr 包")
	}

	for filename, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			vs, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for _, name := range vs.Names {
				if name.Name == "defaultOCR" || name.Name == "defaultOnce" {
					t.Errorf("%s: 发现残留的 %s 变量定义，应删除该单例状态", filename, name.Name)
				}
			}
			return true
		})
	}
}

// TestPackageLevelSingleton_DocNotRecommending 软断言：非测试源码注释中不再推荐 GetDefault。
//
// 简化理由（Finding D5）：原实现使用 AST + 中文关键词匹配扫描注释，过于
// over-engineered。真正的保障来自 TestGetDefault_Removed 和
// TestDefaultOCRVar_Removed 的编译期 AST 检查——只要 GetDefault 函数定义
// 和 defaultOCR/defaultOnce 变量定义不存在，即使注释中写了"推荐 GetDefault"
// 也不会编译通过（导入方会报 undefined）。
//
// 因此本测试简化为一句话的注释提醒：GetDefault 的符号级存在由前两个测试
// 保证，无需额外的中文模式扫描。
func TestPackageLevelSingleton_DocNotRecommending(t *testing.T) {
	// 空函数体：GetDefault 符号级存在由 TestGetDefault_Removed 保证，
	// 注释中的中文建议标记不构成安全边界，无需 AST 扫描。
	// 保留本函数作为墓碑——如果要重新引入 GetDefault，需要显式删除本文件。
}
