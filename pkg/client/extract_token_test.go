// Package client 内部白盒测试。
package client

import (
	"testing"
	"time"
)

// TestExtractTokenFromLocation_ExpiresIn 验证 Location 含 expires_in=N 时
// 返回真实 expiresAt（不再硬编码 now+24h）。
func TestExtractTokenFromLocation_ExpiresIn(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt123&expires_in=3600"
	token, expiresAt := extractTokenFromLocation(loc)
	if token != "jwt123" {
		t.Errorf("token 错：%q", token)
	}
	expected := time.Now().Add(3600 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expiresAt 应 ≈ now+3600s，实际 delta=%v", delta)
	}
}

// TestExtractTokenFromLocation_Exp 验证 Location 含 exp=Unix 时间戳时
// 返回绝对时间。
func TestExtractTokenFromLocation_Exp(t *testing.T) {
	exp := time.Now().Add(2 * time.Hour).Unix()
	loc := "https://example.com/homepage?token=jwt&exp=9999999999" // 已知 2286 年
	token, expiresAt := extractTokenFromLocation(loc)
	if token != "jwt" {
		t.Errorf("token 错：%q", token)
	}
	if !expiresAt.Equal(time.Unix(9999999999, 0)) {
		t.Errorf("exp 解析错误：%v", expiresAt)
	}
	_ = exp
}

// TestExtractTokenFromLocation_Fallback24h 验证无 expires 参数时 fallback 24h。
func TestExtractTokenFromLocation_Fallback24h(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt"
	_, expiresAt := extractTokenFromLocation(loc)
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires 时应 fallback now+24h，实际 delta=%v", delta)
	}
}
