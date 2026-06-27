// Package client 内部白盒测试。
// G2: extractTokenFromReturnData 应解析 returnData.expires_in/exp。
// 历史 bug（一轮仅修了 302 路径，200 路径漏修；round-2 补
// 了对称 warn 但没真解析）：
// - extractTokenFromReturnData 总是返回 time.Now().Add(24*time.Hour)
// - 200 路径永远走 now+24h 兜底，每次合法登录都 warn（auth.go:163-165）
// - 即使 server 真给了 expires_in/exp 字段，extractTokenFromReturnData
// 也完全忽略。
// 修复后：extractTokenFromReturnData 应模仿 parseExpiresMap 解析
// returnData.expires_in（相对秒数）和 returnData.exp（绝对 Unix 时间戳），
// 都缺失才 fallback now+24h。
// 验证策略：直接调 extractTokenFromReturnData，传入含 expires_in / exp /
// 同时含 / 都不含的 returnData，断言 expiresAt 正确。
package client

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// makeUnifiedResp 构造测试用的 UnifiedResponse（UnifiedResponse.ReturnData 是 json.RawMessage）。
func makeUnifiedResp(returnDataJSON string) types.UnifiedResponse {
	raw := json.RawMessage(returnDataJSON)
	return types.UnifiedResponse{
		Code:       1,
		ReturnData: &raw,
	}
}

// TestExtractTokenFromReturnData_ExpiresIn 验证 returnData 含 expires_in
// 时返回真实 expiresAt（now + expires_in 秒），不再硬编码 now+24h。
func TestExtractTokenFromReturnData_ExpiresIn(t *testing.T) {
	resp := makeUnifiedResp(`{"token":"jwt","expires_in":3600}`)
	token, expiresAt, err := extractTokenFromReturnData(resp)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	if token != "jwt" {
		t.Errorf("token 应为 'jwt'，实际: %s", token)
	}
	expected := time.Now().Add(3600 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expiresAt 应 ≈ now+3600s（解析 expires_in），实际 delta=%v", delta)
	}
	// 关键断言：不应是 now+24h 兜底（修复前 bug 症状）
	if time.Until(expiresAt) > 23*time.Hour {
		t.Errorf("expiresAt 居然 ≥ 23h，说明又走 now+24h 兜底了（未解析 expires_in）")
	}
}

// TestExtractTokenFromReturnData_Exp 验证 returnData 含 exp（Unix 秒）时
// 返回绝对时间 time.Unix(n, 0)。
func TestExtractTokenFromReturnData_Exp(t *testing.T) {
	// exp = 1888888888（2030 年附近，足够未来）
	resp := makeUnifiedResp(`{"token":"jwt","exp":1888888888}`)
	token, expiresAt, err := extractTokenFromReturnData(resp)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	if token != "jwt" {
		t.Errorf("token 应为 'jwt'，实际: %s", token)
	}
	if !expiresAt.Equal(time.Unix(1888888888, 0)) {
		t.Errorf("exp 解析错误：期望 time.Unix(1888888888,0)，实际 %v", expiresAt)
	}
}

// TestExtractTokenFromReturnData_Fallback24h 验证 returnData 既无 expires_in
// 也无 exp 时 fallback now+24h（与原行为兼容）。
func TestExtractTokenFromReturnData_Fallback24h(t *testing.T) {
	resp := makeUnifiedResp(`{"token":"jwt"}`)
	_, expiresAt, err := extractTokenFromReturnData(resp)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires_in/exp 时应 fallback now+24h，实际 delta=%v", delta)
	}
}

// TestExtractTokenFromReturnData_ExpiresIn_TakesPriorityOverExp 验证
// expires_in 优先级高于 exp（与 parseExpiresMap 行为对称）。
func TestExtractTokenFromReturnData_ExpiresIn_TakesPriorityOverExp(t *testing.T) {
	// 同时给 expires_in=60 和 exp=1888888888：应取 expires_in
	resp := makeUnifiedResp(`{"token":"jwt","expires_in":60,"exp":1888888888}`)
	_, expiresAt, err := extractTokenFromReturnData(resp)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(60 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expires_in 应优先于 exp，实际 delta=%v", delta)
	}
	// 关键断言：不应是绝对时间 time.Unix(1888888888, 0)
	if strings.Contains(expiresAt.Format(time.RFC3339), "2030") {
		t.Errorf("expires_in 应优先于 exp，实际却走了 exp（绝对时间）")
	}
}
