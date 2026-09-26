package main

import (
	"fmt"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// ─── ErrLoginRejected 归 401（认证拒绝不是 422 未处理实体）───

// TestMapSentinelToHTTPCode_LoginRejected_401 锁定：登录拒绝
// 是明确的认证失败（学号/密码错误），与 login.go 专属中文 401 分支语义
// 对齐——此前 mapSentinelToHTTPCode 与 ErrBusinessRejected 同列归 422，
// 同一「登录拒绝」语义在 login 命令走 401、其余命令走 422 双套映射。
func TestMapSentinelToHTTPCode_LoginRejected_401(t *testing.T) {
	err := fmt.Errorf("登录失败: %w", client.ErrLoginRejected)
	if got := mapSentinelToHTTPCode(err); got != 401 {
		t.Errorf("ErrLoginRejected 应映射 401，实际 %d（此前与业务拒绝同列 422）", got)
	}
}

// TestMapSentinelToHTTPCode_BusinessRejected_Still422 反向锁定：去掉
// ErrLoginRejected 后，纯业务拒绝仍必须保持 422 档位（无行为回归）。
func TestMapSentinelToHTTPCode_BusinessRejected_Still422(t *testing.T) {
	err := fmt.Errorf("平台拒绝: %w", client.ErrBusinessRejected)
	if got := mapSentinelToHTTPCode(err); got != 422 {
		t.Errorf("ErrBusinessRejected 应保持 422，实际 %d", got)
	}
}

// TestMapSentinelToHTTPCode_LoginRejectedInErrorChain_401 验证嵌套错误链中
// ErrLoginRejected 也能被 errors.Is 命中归 401（与 L1 汇总链同构）。
func TestMapSentinelToHTTPCode_LoginRejectedInErrorChain_401(t *testing.T) {
	inner := fmt.Errorf("SSO 拒绝: %w", client.ErrLoginRejected)
	outer := fmt.Errorf("认证失败: %w", inner)
	if got := mapSentinelToHTTPCode(outer); got != 401 {
		t.Errorf("错误链含 ErrLoginRejected 应映射 401，实际 %d", got)
	}
}
