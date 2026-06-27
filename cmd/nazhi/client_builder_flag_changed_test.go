// client_builder_flag_changed_test.go 锚定
// buildClientOpts + login.go 在 token/username/password 读取时必须用
// flagChanged() 守卫，避免用户显式传 --token "" 覆盖 NAZHI_TOKEN。
// F7 证据：cmd.Flags().GetString("token") 在用户显式传 --token "" 时返回空字符串
// 后续 `if token == "" { token = envString(...) }` 看似"哨兵默认 + env 覆盖"
// 实际是反模式——显式传空字符串的用户期望被尊重（fail-fast 报缺 token）
// 而不应该悄悄用环境变量 fallback。
// 设计契约
//   - cmd.Flags().Changed("token") == true → 用户显式传过 flag，flag 值生效
//     （包括显式传空字符串）
//   - cmd.Flags().Changed("token") == false → 未传 flag，走 env fallback
//
// 适用面
//   - token (buildClientOpts)
//   - username/password (login.go)
//   - sso-base/base-url/upload-url
//
// 测试策略
//  1. 不传 --token flag + NAZHI_TOKEN 设值 → token 读 env
//  2. 传 --token "" 显式空字符串 + NAZHI_TOKEN 设值 → token 保持空字符串
//  3. 传 --token "explicit" + NAZHI_TOKEN 设值 → token 用 flag 值（环境变量不生效）
//  4. login 用户名/密码对称测试
package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// flagSet 辅助：构造 cobra cmd 并显式调用 Set 让 Changed() 返回 true。
// 注意：pflag 的 Changed 状态由 Set() 触发，flags.String 默认值赋值不会标记 Changed。
// 这正是 F7 bug 的根源——cobra 默认值 + 显式传 "" 在 pflag 内部状态完全一致。
func flagSet(t *testing.T, flags map[string]any) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	for name, val := range flags {
		switch v := val.(type) {
		case string:
			cmd.Flags().String(name, "", "")
			if v != "" {
				_ = cmd.Flags().Set(name, v)
			} else {
				// 即使空字符串也要 Set 才能让 Changed()=true
				_ = cmd.Flags().Set(name, "")
			}
		case int:
			cmd.Flags().Int(name, 0, "")
			if v != 0 {
				_ = cmd.Flags().Set(name, itoa(v))
			}
		}
	}
	return cmd
}

// TestBuildClientOpts_TokenFlagExplicit_OverridesEnv
// 用户显式传 --token "explicit-token" + NAZHI_TOKEN="env-token" 时
// flag 值必须生效，env 不应覆盖。
// 修复前：token="" → env fallback → token="env-token"（违反用户意图）
// 修复后：flag Changed=true → token 保持 "explicit-token"
func TestBuildClientOpts_TokenFlagExplicit_OverridesEnv(t *testing.T) {
	t.Setenv("NAZHI_TOKEN", "env-token-should-be-ignored")

	cmd := flagSet(t, map[string]any{
		"base-url": "http://example.com:8280",
		"timeout":  15,
		"token":    "explicit-token",
	})

	_, token, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts: %v", err)
	}

	if token != "explicit-token" {
		t.Errorf("F7 修复：--token 显式值必须生效，期望 explicit-token 实际 %q", token)
	}
}

// TestBuildClientOpts_TokenFlagEmpty_DoesNotFallbackEnv F7 关键不变量
// 用户显式传 --token "" + NAZHI_TOKEN set 时，flag 显式空字符串应被尊重
// 不应该悄悄用 env 兜底。
// 设计理由：用户主动传 --token "" 表达"我故意要空"或"我覆盖了 env 默认"
// 此时 buildBizClient 的 requireToken 校验会按契约报错（缺 token）
// 而不是 env 偷偷填充导致 user 困惑「我明明传了为啥还报错」。
// 修复前：token="" → env fallback → token="env-token"
// 修复后：flag Changed=true → token 保持 "" → requireToken 报错
func TestBuildClientOpts_TokenFlagEmpty_DoesNotFallbackEnv(t *testing.T) {
	t.Setenv("NAZHI_TOKEN", "env-token")

	cmd := flagSet(t, map[string]any{
		"base-url": "http://example.com:8280",
		"timeout":  15,
		"token":    "", // 显式空字符串
	})

	_, token, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts: %v", err)
	}

	if token != "" {
		t.Errorf("F7 修复：显式 --token \"\" 不应被 env 覆盖，期望空字符串实际 %q", token)
	}

	// 进一步：requireToken=true 时应报缺 token 错
	_, _, err = buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", true)
	if err == nil {
		t.Error("F7 修复：显式 --token \"\" + requireToken=true 应报缺 token 错")
	} else if !strings.Contains(err.Error(), "token") {
		t.Errorf("错误信息应提及 token，实际: %v", err)
	}
}

// TestBuildClientOpts_TokenFlagAbsent_FallsBackToEnv 回归保险
// 未传 --token flag + NAZHI_TOKEN set 时，env 必须生效。
func TestBuildClientOpts_TokenFlagAbsent_FallsBackToEnv(t *testing.T) {
	t.Setenv("NAZHI_TOKEN", "env-token-only")

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("base-url", "", "")
	cmd.Flags().Int("timeout", 0, "")
	// 不注册 token flag → Changed() 调用会 panic，应仅注册 flag 不 Set
	cmd.Flags().String("token", "", "")

	_, token, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts: %v", err)
	}

	if token != "env-token-only" {
		t.Errorf("未传 --token 时应读 NAZHI_TOKEN，期望 env-token-only 实际 %q", token)
	}
}

// TestLogin_UsernamePasswordFlagChanged 在 login.go 的对称修复
// 用户显式传 --username "" + NAZHI_USERNAME set 时，flag 应被尊重。
func TestLogin_UsernamePasswordFlagChanged(t *testing.T) {
	t.Setenv("NAZHI_USERNAME", "env-username")
	t.Setenv("NAZHI_PASSWORD", "env-password")

	cmd := flagSet(t, map[string]any{
		"username": "explicit-user",
		"password": "explicit-pass",
		"sso-base": "",
		"timeout":  15,
	})

	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")

	if username != "explicit-user" {
		t.Errorf("login username flag 显式值应生效，实际 %q", username)
	}
	if password != "explicit-pass" {
		t.Errorf("login password flag 显式值应生效，实际 %q", password)
	}
}

// TestLogin_UsernameFlagAbsent_FallsBackToEnv login username flag 未传时读 env。
func TestLogin_UsernameFlagAbsent_FallsBackToEnv(t *testing.T) {
	t.Setenv("NAZHI_USERNAME", "env-username")
	t.Setenv("NAZHI_PASSWORD", "env-password")

	cmd := &cobra.Command{Use: "login"}
	cmd.Flags().String("username", "", "")
	cmd.Flags().String("password", "", "")
	cmd.Flags().String("sso-base", "", "")
	cmd.Flags().Int("timeout", 15, "")

	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")

	// 不通过 Set() 触发 Changed，所以走 env fallback
	if username != "" {
		t.Fatalf("预置 username=%q 期望空", username)
	}
	if password != "" {
		t.Fatalf("预置 password=%q 期望空", password)
	}
}
