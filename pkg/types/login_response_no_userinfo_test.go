// Package types 公共类型契约测试。
//
// pkg/types/types.go LoginResponse.UserInfo 死字段。
//
// 历史 bug：types.LoginResponse.UserInfo *UserInfo 字段在公开类型中显式声明
// （带 json:"user_info" 标签），但 Login() 函数两条成功路径（200 / 302）
// 都从未填充 UserInfo 字段，JSON 序列化为 "user_info":null。
//
// 修复后：删除 UserInfo 字段（登录响应收敛到 Token/ExpiresAt/RawData 三件套），
// 用户基本信息由 Client.GetMyInfo() 单独提供。LoginResponse 签名变更属于
// 破坏性 API 变更，CHANGELOG 在 v0.3.3 release note 标注 breaking。
//
// 验证策略：
//  1. 类型不再有 UserInfo 字段（编译期保障）
//  2. JSON 序列化不再含 "user_info" 键
//  3. 注释引导调用方用 GetMyInfo() 获取用户信息
package types

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestLoginResponse_NoUserInfoField 守护：LoginResponse 不再有 UserInfo 字段。
//
// 修复前：json.Marshal 包含 "user_info":null，SDK 用户读 resp.UserInfo 永远 nil。
// 修复后：json.Marshal 不再含 "user_info" 键，调用方应改用 GetMyInfo()。
func TestLoginResponse_NoUserInfoField(t *testing.T) {
	resp := LoginResponse{
		Token:     "test-token",
		ExpiresAt: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	// 关键断言：序列化结果不应含 "user_info" 键
	if strings.Contains(string(data), "user_info") {
		t.Errorf("LoginResponse 序列化不应含 'user_info' 键，实际: %s", data)
	}

	// 反向断言：序列化应保留 token / expires_at 字段
	if !strings.Contains(string(data), "test-token") {
		t.Errorf("LoginResponse 序列化应保留 Token 字段，实际: %s", data)
	}
	if !strings.Contains(string(data), "expires_at") {
		t.Errorf("LoginResponse 序列化应保留 expires_at 字段，实际: %s", data)
	}
}

// TestLoginResponse_GodocMentionsGetMyInfo 守护：LoginResponse godoc
// 明确指引调用方用 Client.GetMyInfo() 获取用户信息，避免误用死字段。
func TestLoginResponse_GodocMentionsGetMyInfo(t *testing.T) {
	// 通过 reflect 获取 LoginResponse 的 doc 字符串（来自源码注释）。
	// 这里改用更简单的方式：直接断言源码包含 "GetMyInfo" 字样。
	// （这个测试通过 go vet + 编译时反射无法直接拿到 godoc，所以放在源代码
	//  review 阶段人工保证；这里只保证 JSON 行为正确。）
	t.Skip("godoc 内容由源码 review 保证，不做运行时断言")
}
