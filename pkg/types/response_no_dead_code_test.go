// Package types 公共类型契约测试 — 死代码守卫（F5 + F6）。
//
// 守卫两项：
//   - F5: pkg/types/response.go 不应再定义 DecodeUnified（候选 #3 二合一原语，0 生产调用方）
//   - F6: UnifiedResponse 不应再保留 DataInt 字段（仅测试自引用，0 生产调用方）
//
// 防止「已删除的死代码」通过 git blame / 旧 commit 复活。
package types

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestNoDecodeUnified 守护：DecodeUnified 不应在 response.go 中存在。
//
// 验证策略：AST 解析 pkg/types/response.go，扫描顶层函数声明，
// 找到 DecodeUnified 即 FAIL。
func TestNoDecodeUnified(t *testing.T) {
	// 用 AST 检查同包源码文件——相对当前测试包目录解析
	src, err := filepath.Abs("response.go")
	if err != nil {
		t.Fatalf("filepath.Abs 失败: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", src, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue // 跳过方法，只看顶层函数
		}
		if fn.Name.Name == "DecodeUnified" {
			t.Errorf("F5 回归：pkg/types/response.go 仍存在 DecodeUnified 函数（应已删除）")
		}
	}
}

// TestNoDataIntField 守护：UnifiedResponse 不应再保留 DataInt 字段。
//
// 验证策略：
//  1. 反射：UnifiedResponse 的 struct field 列表不含 DataInt
//  2. JSON 序列化：DataInt 字段不应出现在 marshal 后的 JSON 中
func TestNoDataIntField(t *testing.T) {
	t.Run("反射字段检查", func(t *testing.T) {
		resp := UnifiedResponse{}
		rt := reflect.TypeOf(resp)
		if _, found := rt.FieldByName("DataInt"); found {
			t.Error("F6 回归：UnifiedResponse 仍存在 DataInt 字段（应已删除）")
		}
	})

	t.Run("JSON 序列化检查", func(t *testing.T) {
		// 即使有人加回 DataInt，序列化也会暴露 dataInt 键
		resp := UnifiedResponse{Code: 1}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("json.Marshal 失败: %v", err)
		}
		if strings.Contains(string(data), "dataInt") {
			t.Errorf("F6 回归：UnifiedResponse 序列化仍含 dataInt 键，实际: %s", data)
		}
	})
}
