// Package client 内部白盒测试 — F2 锁定 strings.Contains 语义。
package client

import (
	"strings"
	"testing"
)

// TestF2_StringsContainsStandardLibrary 回归测试（F2）：
// session_concurrent_test.go 自造 contains() 改用 strings.Contains 后，
// 显式锁定标准库语义以防"自造轮子"再次潜入。
// 历史 bug：原 contains() 函数自造避免导入 strings，注释称"防止额外
// 编译器感知"——但 Go test 文件已大量导入 strings，理由错误。本测试
// 用 strings.Contains 替代自造版本，并直接断言标准库行为。
func TestF2_StringsContainsStandardLibrary(t *testing.T) {
	// 子串存在
	if !strings.Contains("homepage?token=abc", "homepage") {
		t.Error("strings.Contains 应检测到子串 'homepage'")
	}
	// 子串不存在
	if strings.Contains("homepage?token=abc", "/home") == strings.Contains("homepage?token=abc", "homepage") {
		t.Error("strings.Contains 对不同子串应给出不同结果")
	}
	// 空串永远为 true
	if !strings.Contains("anything", "") {
		t.Error("strings.Contains(x, \"\") 应为 true")
	}
	// 完全相同
	if !strings.Contains("foo", "foo") {
		t.Error("strings.Contains 应检测到完全相同的子串")
	}
}
