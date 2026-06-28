// Package tokenparse 的单元测试，遵循 RED-GREEN-REFACTOR TDD 纪律。
package tokenparse

import (
	"encoding/json"
	"testing"
	"time"
)

// ─── ExtractFromLocation 测试 ───

func TestExtractFromLocation_BasicToken(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt123"
	token, _, err := ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "jwt123" {
		t.Errorf("token 应为 'jwt123'，实际: %q", token)
	}
}

func TestExtractFromLocation_ExpiresIn(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt123&expires_in=3600"
	token, expiresAt, err := ExtractFromLocation(loc)
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

func TestExtractFromLocation_Exp(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt&exp=9999999999"
	token, expiresAt, err := ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "jwt" {
		t.Errorf("token 错：%q", token)
	}
	if !expiresAt.Equal(time.Unix(9999999999, 0)) {
		t.Errorf("exp 解析错误：%v", expiresAt)
	}
}

func TestExtractFromLocation_Fallback24h(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt"
	_, expiresAt, err := ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires 时应 fallback now+24h，实际 delta=%v", delta)
	}
}

func TestExtractFromLocation_FragmentFallback(t *testing.T) {
	loc := "https://example.com/homepage#token=fragment-jwt"
	token, _, err := ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "fragment-jwt" {
		t.Errorf("fragment token 错：%q", token)
	}
}

func TestExtractFromLocation_MalformedURL_ReturnsError(t *testing.T) {
	loc := "http://[::1"
	token, _, err := ExtractFromLocation(loc)
	if err == nil {
		t.Fatal("畸形 URL 应返回 error，实际 nil")
	}
	if token != "" {
		t.Errorf("畸形 URL 应返回空 token，实际 %q", token)
	}
	// 死代码删除：tokenparse 包不再定义 ErrLocationParseFailed sentinel。
	// 错误（裸 url.Parse error）仍正常返回，调用方只需 err != nil 即可识别。
}

func TestExtractFromLocation_QueryTakesPriorityOverFragment(t *testing.T) {
	loc := "https://example.com/homepage?token=query-jwt#token=fragment-jwt"
	token, _, err := ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "query-jwt" {
		t.Errorf("query 应优先于 fragment，实际 %q", token)
	}
}

// TestExtractFromLocation_FragmentTokenPreserved 验证 fragment 中的 token 被正确提取。
func TestExtractFromLocation_FragmentTokenPreserved(t *testing.T) {
	loc := "https://example.com/homepage#token=bare-jwt-value"
	token, _, err := ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "bare-jwt-value" {
		t.Errorf("fragment token 错：want %q got %q", "bare-jwt-value", token)
	}
}

// TestExtractFromLocation_EmptyLocation 验证空字符串返回空 token 且无 error。
// url.Parse("") 返回空 URL 结构（无 error），原 auth.go extractTokenFromLocation
// 同样返回 ("", now+24h, nil)，行为 100% 兼容。
func TestExtractFromLocation_EmptyLocation(t *testing.T) {
	token, _, err := ExtractFromLocation("")
	if err != nil {
		t.Fatalf("空字符串 Location 不应返回 error: %v", err)
	}
	if token != "" {
		t.Errorf("空字符串 Location 应返回空 token，实际 %q", token)
	}
}

// ─── ExtractFromReturnData 测试 ───

func TestExtractFromReturnData_BasicToken(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt-no-exp"}`)
	token, _, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("合法 returnData 不应返回 error: %v", err)
	}
	if token != "jwt-no-exp" {
		t.Errorf("token 应为 'jwt-no-exp'，实际 %q", token)
	}
}

func TestExtractFromReturnData_ExpiresIn(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","expires_in":3600}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(3600 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expiresAt 应 ≈ now+3600s，实际 delta=%v", delta)
	}
	if time.Until(expiresAt) > 23*time.Hour {
		t.Errorf("expiresAt 居然 ≥ 23h，说明又走 now+24h 兜底了（未解析 expires_in）")
	}
}

func TestExtractFromReturnData_Exp(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","exp":1888888888}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	if !expiresAt.Equal(time.Unix(1888888888, 0)) {
		t.Errorf("exp 解析错误：期望 time.Unix(1888888888,0)，实际 %v", expiresAt)
	}
}

func TestExtractFromReturnData_Fallback24h(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt"}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires_in/exp 时应 fallback now+24h，实际 delta=%v", delta)
	}
}

func TestExtractFromReturnData_ExpiresIn_TakesPriorityOverExp(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","expires_in":60,"exp":1888888888}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(60 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expires_in 应优先于 exp，实际 delta=%v", delta)
	}
}

func TestExtractFromReturnData_EmptyRaw(t *testing.T) {
	_, _, err := ExtractFromReturnData(nil)
	if err == nil {
		t.Fatal("空 RawMessage 应返回 error，实际 nil")
	}
	_, _, err = ExtractFromReturnData(json.RawMessage(""))
	if err == nil {
		t.Fatal("空字符串 RawMessage 应返回 error，实际 nil")
	}
}

func TestExtractFromReturnData_MissingToken(t *testing.T) {
	raw := json.RawMessage(`{"other":"value"}`)
	_, _, err := ExtractFromReturnData(raw)
	if err == nil {
		t.Fatal("缺 token 字段应返回 error，实际 nil")
	}
}

func TestExtractFromReturnData_TokenTypeMismatch(t *testing.T) {
	raw := json.RawMessage(`{"token":123}`)
	_, _, err := ExtractFromReturnData(raw)
	if err == nil {
		t.Fatal("token 类型异常应返回 error，实际 nil")
	}
}

func TestExtractFromReturnData_EmptyToken(t *testing.T) {
	raw := json.RawMessage(`{"token":""}`)
	_, _, err := ExtractFromReturnData(raw)
	if err == nil {
		t.Fatal("token 空字符串应返回 error，实际 nil")
	}
}

// ─── parseExpiresMap 间接测试 ───
//
// parseExpiresMap 是私有函数，通过 ExtractFromReturnData 间接覆盖。
// 所有测试用例都带上 "token":"jwt" 避免被 ExtractFromReturnData 的 token 守卫阻断。

func TestParseExpiresMap_ExpiresInTakesPriority(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","expires_in":120,"exp":1888888888}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(120 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expires_in 应优先于 exp，实际 delta=%v", delta)
	}
}

func TestParseExpiresMap_InvalidExpiresInFallsBackToExp(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","expires_in":"not-a-number","exp":1888888888}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	if !expiresAt.Equal(time.Unix(1888888888, 0)) {
		t.Errorf("expires_in 无效时应 fallback 到 exp，实际 %v", expiresAt)
	}
}

func TestParseExpiresMap_AllInvalidFallsBackTo24h(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","expires_in":"junk","exp":"junk"}`)
	_, expiresAt, err := ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(DefaultTokenTTL)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expires_in/exp 都无效时应 fallback 24h，实际 delta=%v", delta)
	}
}

// ─── errors.Is / Unwrap 链路测试 ───
//
// 死代码删除：tokenparse 包不再导出 ErrLocationParseFailed sentinel。
// 畸形 URL 走裸 url.Parse error，调用方用 err != nil 即可识别。
// 原 TestWrapLocationParseErr_PreservesInnerError 因依赖被删除的 sentinel
// 整体移除。

// ─── DefaultTokenTTL 常量回归 ───

func TestDefaultTokenTTL(t *testing.T) {
	if DefaultTokenTTL != 24*time.Hour {
		t.Errorf("DefaultTokenTTL 应为 24h，实际 %v", DefaultTokenTTL)
	}
}
