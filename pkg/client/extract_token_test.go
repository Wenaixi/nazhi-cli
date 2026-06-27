// Package client 内部白盒测试。
package client

import (
	"errors"
	"testing"
	"time"
)

// TestExtractTokenFromLocation_ExpiresIn 验证 Location 含 expires_in=N 时
// 返回真实 expiresAt（不再硬编码 now+24h）。
func TestExtractTokenFromLocation_ExpiresIn(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt123&expires_in=3600"
	token, expiresAt, err := extractTokenFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
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
	token, expiresAt, err := extractTokenFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
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
	_, expiresAt, err := extractTokenFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires 时应 fallback now+24h，实际 delta=%v", delta)
	}
}

// F2-EXTRACT-TOKEN-ASYM RED 测试：畸形 URL 返回 error，与 extractTokenFromReturnData
// 的错误传播契约对称。
// 用例：`http://[::1` 是缺少闭合 `]` 的 IPv6 字面量，net/url 必返回 parse error。
// 修复前：静默返回 ("", now+24h) — 错误吞掉，调用方看到「未找到 token」。
// 修复后：返回包装 ErrLocationParseFailed 的 error。
func TestExtractTokenFromLocation_MalformedURL_ReturnsError(t *testing.T) {
	loc := "http://[::1"
	token, _, err := extractTokenFromLocation(loc)
	if err == nil {
		t.Fatal("畸形 URL 应返回 error，实际 nil")
	}
	if token != "" {
		t.Errorf("畸形 URL 应返回空 token，实际 %q", token)
	}
	if !errors.Is(err, ErrLocationParseFailed) {
		t.Errorf("error 应包装 ErrLocationParseFailed，实际 %v", err)
	}
}

// F10-FRAGMENT-URLDECODE RED 测试：fragment 中的 token= 值需 URL 解码。
// 历史：strings.Split + TrimPrefix 只做字符串裁剪，JWT 含 + / = 等 URL 保留
// 字符时会损坏 token。修复后 url.QueryUnescape 还原原始 base64 JWT。
// 用例：eyJ%2Bxxx%3D 解码后应为 eyJ+xxx=。
func TestExtractTokenFromFragment_URLEncodedValue(t *testing.T) {
	fragment := "token=eyJ%2Bxxx%3D"
	got := extractTokenFromFragment(fragment)
	want := "eyJ+xxx="
	if got != want {
		t.Errorf("URL 解码错：want %q got %q", want, got)
	}
}

// F10 边界：URL 解码失败时 fallback 到原始 value（best-effort 语义）。
func TestExtractTokenFromFragment_BadEncodingFallsBackToRaw(t *testing.T) {
	fragment := "token=%ZZ" // 非法百分号编码
	got := extractTokenFromFragment(fragment)
	if got != "%ZZ" {
		t.Errorf("bad encoding 应 fallback 原始 value，want %q got %q", "%ZZ", got)
	}
}

// F10 普通用例：无 URL 编码时透传。
func TestExtractTokenFromFragment_PlainValue(t *testing.T) {
	fragment := "token=jwt123&other=x"
	got := extractTokenFromFragment(fragment)
	if got != "jwt123" {
		t.Errorf("plain value 错：got %q", got)
	}
}
