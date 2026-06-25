// Package ocr 内部白盒测试：I1 finding 验证。
//
// Finding I1：GetDefault() / defaultOCR / defaultOnce 是 0 调用方的进程级单例，
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
	"strings"
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
				t.Errorf("%s: 发现残留的 GetDefault 顶层函数定义，I1 修复要求删除该单例 API", filename)
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
					t.Errorf("%s: 发现残留的 %s 变量定义，I1 修复要求删除该单例状态", filename, name.Name)
				}
			}
			return true
		})
	}
}

// TestPackageLevelSingleton_NotReferenced 软断言：源文件注释/字符串中不应再
// 推荐使用 GetDefault（防止文档回潮）。
//
// 注意：本测试只扫描**非测试**源文件，因为测试文件可能需要保留历史 mention
// 用于验证目的。
func TestPackageLevelSingleton_DocNotRecommending(t *testing.T) {
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
		// 跳过本测试自身，避免误报
		if strings.Contains(filename, "ocr_default_removed_test.go") {
			continue
		}
		// 跳过 _test.go 文件
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		for _, group := range file.Comments {
			text := group.Text()
			// 扫描关键词：注释中若仍在"推荐"GetDefault 使用，则告警
			if strings.Contains(text, "GetDefault") && (strings.Contains(text, "推荐") ||
				strings.Contains(text, "建议") || strings.Contains(text, "应该用") ||
				strings.Contains(text, "用 GetDefault") || strings.Contains(text, "使用 GetDefault")) {
				t.Errorf("%s: 注释中仍在推荐使用 GetDefault，I1 修复要求移除该建议:\n  %s",
					filename, strings.TrimSpace(text))
			}
		}
	}
}
