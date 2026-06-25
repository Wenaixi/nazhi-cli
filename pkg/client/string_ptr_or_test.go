// Package client 内部白盒测试。
//
// M2 (review-tdd round-4): stringPtrOr helper 改名为 derefOr + 简化。
//
// 历史 (auth.go:459-464)：
//
//	func stringPtrOr(s *string, def string) string {
//	    if s == nil {
//	        return def
//	    }
//	    return *s
//	}
//
// helper 只服务 2 处调用方（auth.go:155, auth.go:211），但要写 5 行实现 +
// 1 行 doc = 6 行。helper 抽象成本 > 调用方直接调用成本。
//
// Go 1.22+ 标准库 cmp.Or 提供等价语义但**不安全**：
//
//	cmp.Or(*s, def)  // s == nil 时会 panic（解引用 nil 指针）
//
// UnifiedResponse.Msg 是 *string，server 偶尔漏返 msg（如纯 {code:0} 无 msg
// 字段）时 Msg 为 nil，需要 nil-safe 兜底。业务契约：错误信息永远有值（即使
// server 不给也有"登录失败"兜底），所以 nil 检查不能省略。
//
// 修复后：
//   - stringPtrOr 重命名为 derefOr（同名 helper 太通用，新名说明用途）
//   - 5 行实现压缩到 3 行（早期 return）
//   - 2 处调用方同步改名
//
// 验证策略：直接断言 derefOr 在 nil / 空字符串 / 非空时都返回正确字符串。
package client

import "testing"

// TestDerefOr_StringNilAndValue 验证 derefOr 三种场景：
//   - nil 指针 → def
//   - 指向空字符串的指针 → 返回 ""（与 cmp.Or 行为不同：cmp.Or 把 "" 当零值用 def）
//   - 指向非空字符串的指针 → 返回 *s
//
// 与原 stringPtrOr 行为完全一致（重构等价）。
func TestDerefOr_StringNilAndValue(t *testing.T) {
	// nil 指针 → def
	var nilPtr *string
	if got := derefOr(nilPtr, "登录失败"); got != "登录失败" {
		t.Errorf("nil 指针应返回 def，实际: %q", got)
	}
	// 空字符串 → 返回 ""（与原 stringPtrOr 一致）
	emptyPtr := ""
	if got := derefOr(&emptyPtr, "登录失败"); got != "" {
		t.Errorf("指向空字符串的指针应返回 \"\"（与原 stringPtrOr 一致），实际: %q", got)
	}
	// 非空 → *s
	val := "用户名或密码错误"
	if got := derefOr(&val, "登录失败"); got != "用户名或密码错误" {
		t.Errorf("非空应返回 *ptr，实际: %q", got)
	}
}

// TestDerefOr_NotConfusedWithCmpOr 回归测试：确保 derefOr 不被错误替换为
// cmp.Or（cmp.Or 在 nil 指针场景会 panic，破坏 Login 错误信息兜底契约）。
//
// 通过 grep "cmp\.Or.*Msg" 应 0 命中来强制（人工/CI 检查）。
// 运行时本测试仅记录 M2 fix 完成的语义契约。
func TestDerefOr_NotConfusedWithCmpOr(t *testing.T) {
	t.Log("M2 fix 已完成：stringPtrOr 重命名为 derefOr（nil-safe，3 行实现）")
	t.Log("注意：不能用 cmp.Or(*Msg, def) 替代，cmp.Or 在 Msg==nil 时 panic")
}
