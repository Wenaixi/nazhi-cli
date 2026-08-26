package logx

import "testing"

// TestRedactBodyThenTruncate_Order 锁定 HTTP-1 契约：
// 脱敏必须先于截断——先截断再脱敏会让跨截断边界的敏感值前缀泄漏
// （kvRe 需要闭合引号、tokenQueryRe 需要 key= 形态，值被切腰后正则失配）。
// 本测试只断言新函数 RedactBodyThenTruncate 的行为：敏感值不泄漏 + 输出截断。
func TestRedactBodyThenTruncate_Order(t *testing.T) {
	longBody := `{"code":1,"msg":"ok","data":{"token":"` + makeToken(100) + `"}}`
	got := RedactBodyThenTruncate([]byte(longBody), 100)
	if contains(got, "tok_") {
		t.Errorf("RedactBodyThenTruncate 泄漏 token: %q", got)
	}
	if len(got) > 103 {
		t.Errorf("RedactBodyThenTruncate 输出超长: len=%d", len(got))
	}
	// 重要：确保被截断的不是敏感值本身（脱敏先发生，掩码计数应保留）
	if !contains(got, "***") {
		t.Errorf("脱敏应已生效（输出应含掩码），实际: %q", got)
	}
}

func makeToken(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[i%16]
	}
	return "tok_" + string(b)
}

func contains(s, sub string) bool {
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
