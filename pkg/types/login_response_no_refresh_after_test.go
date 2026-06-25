// Package types 公共类型契约测试 — RefreshAfter 死字段删除守卫。
//
// H1 (review-tdd 四轮): pkg/types/types.go LoginResponse.RefreshAfter 死字段。
//
// 历史 bug：types.LoginResponse.RefreshAfter time.Time 字段在公开类型中显式声明
// （带 json:"refresh_after" 标签），但全仓 0 引用 — 没有任何代码读或写该字段。
//
// 修复后：删除 RefreshAfter 字段，LoginResponse 收敛到 Token/ExpiresAt/RawData
// 三件套（实际被填充的字段）。JSON 序列化不再含 "refresh_after" 键。
//
// 验证策略：
//  1. 类型不再有 RefreshAfter 字段（编译期保障 — 无法通过 reflect 引用）
//  2. JSON 序列化不再含 "refresh_after" 键
package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestLoginResponse_NoRefreshAfterField 守护 H1 修复：LoginResponse 不再有 RefreshAfter 字段。
//
// 修复前：json.Marshal 包含 "refresh_after":"0001-01-01T00:00:00Z"，SDK 用户读
//
//	resp.RefreshAfter 永远零值。
//
// 修复后：json.Marshal 不再含 "refresh_after" 键，调用方应通过 ExpiresAt 自行判断刷新时机。
func TestLoginResponse_NoRefreshAfterField(t *testing.T) {
	resp := LoginResponse{
		Token:     "test-token",
		ExpiresAt: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	// 关键断言：序列化结果不应含 "refresh_after" 键
	if strings.Contains(string(data), "refresh_after") {
		t.Errorf("LoginResponse 序列化不应含 'refresh_after' 键，实际: %s", data)
	}

	// 反向断言：序列化应保留 token / expires_at 字段（确保其它活跃字段未受影响）
	if !strings.Contains(string(data), "test-token") {
		t.Errorf("LoginResponse 序列化应保留 Token 字段，实际: %s", data)
	}
	if !strings.Contains(string(data), "expires_at") {
		t.Errorf("LoginResponse 序列化应保留 expires_at 字段，实际: %s", data)
	}
}
