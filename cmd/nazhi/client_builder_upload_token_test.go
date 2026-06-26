// client_builder_upload_token_test.go F16 修复锚定：
// buildClientOpts 在 urlType=="upload" 路径必须短路 token 读取。
//
// F16 证据：原 buildClientOpts 无条件读 NAZHI_TOKEN（不依赖 urlType），
// 即使 file upload 命令显式不提供 --token flag，NAZHI_TOKEN 环境变量
// 仍会被注入到 pendingToken → syncCookieToken 写 cookie jar 到 sso/api
// 域，违反 fileUploadCmd 文档契约「本命令不接受 --token 参数」。
//
// 修复：urlType=="upload" 分支短路 token 读取，跳过 cmd.Flags().GetString
// 和 envString("NAZHI_TOKEN")，opts 列表不 append WithToken。
//
// 测试策略：
//  1. 设置 NAZHI_TOKEN 为非空字符串
//  2. 调 buildClientOpts(cmd, "upload", ...)
//  3. 断言返回的 token 字符串 == ""（关键契约）
//  4. 回归：base 路径不受影响，token 必须从 NAZHI_TOKEN 读到
package main

import (
	"testing"
)

// TestBuildClientOpts_UploadIgnoresNAZHI_TOKEN 设置 NAZHI_TOKEN 后跑
// urlType="upload"，验证返回 token 字符串为空（核心契约）。
func TestBuildClientOpts_UploadIgnoresNAZHI_TOKEN(t *testing.T) {
	// 准备：设置 NAZHI_TOKEN 应被 upload 路径短路
	t.Setenv("NAZHI_TOKEN", "should-be-ignored-token-12345")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"upload-url": "http://example.com:8080",
		"timeout":    30,
	})

	_, token, err := buildClientOpts(cmd, "upload", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts 不应返回 error，实际: %v", err)
	}

	if token != "" {
		t.Errorf("F16 回归：urlType=upload 路径返回 token=%q，期望空字符串。"+
			"file upload 命令不接受 token，NAZHI_TOKEN 应被短路，"+
			"否则 syncCookieToken 会写 cookie 到 sso/api 域",
			token)
	}
}

// TestBuildClientOpts_UploadIgnoresExplicitTokenFlag --token flag 在 upload 路径
// 也应被短路（file upload 命令根本不注册 --token flag，但 buildClientOpts 内部
// 仍可能从 cmd.Flags() 读到不存在的 flag 的零值）。
func TestBuildClientOpts_UploadIgnoresExplicitTokenFlag(t *testing.T) {
	// 不设 NAZHI_TOKEN，但 cmd flags 含 token=explicit-token
	cmd := makeTestCmdWithFlags(t, map[string]any{
		"upload-url": "http://example.com:8080",
		"timeout":    30,
		"token":      "explicit-token-should-also-be-ignored",
	})

	_, token, err := buildClientOpts(cmd, "upload", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts: %v", err)
	}

	if token != "" {
		t.Errorf("F16 回归：urlType=upload 路径读到 --token=%q，期望空字符串",
			token)
	}
}

// TestBuildClientOpts_BaseRespectsNAZHI_TOKEN 回归保险：base 路径不受影响。
func TestBuildClientOpts_BaseRespectsNAZHI_TOKEN(t *testing.T) {
	t.Setenv("NAZHI_TOKEN", "valid-base-token")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"base-url": "http://example.com:8280",
		"timeout":  15,
	})

	_, token, err := buildClientOpts(cmd, "base", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts: %v", err)
	}

	if token != "valid-base-token" {
		t.Errorf("base 路径应读 NAZHI_TOKEN，期望 valid-base-token 实际 %q", token)
	}
}

// TestBuildClientOpts_SsoIgnoresNAZHI_TOKEN 回归保险：sso 路径也不读 token
// （login/school 不需要 token）。这是 SSOflow 设计契约。
func TestBuildClientOpts_SsoIgnoresNAZHI_TOKEN(t *testing.T) {
	t.Setenv("NAZHI_TOKEN", "should-be-ignored-for-sso")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "http://example.com",
		"timeout":  15,
	})

	_, token, err := buildClientOpts(cmd, "sso", "NAZHI_TIMEOUT", false)
	if err != nil {
		t.Fatalf("buildClientOpts: %v", err)
	}

	if token != "" {
		t.Errorf("sso 路径不应读 NAZHI_TOKEN，实际 token=%q", token)
	}
}
