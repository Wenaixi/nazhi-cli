package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// makeTestCmdWithFlags 构造一个临时 cobra 命令并附加指定 flag，模拟 buildClient
// 内部的 cmd.Flags().Get* 调用。
func makeTestCmdWithFlags(t *testing.T, flags map[string]any) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	for name, val := range flags {
		switch v := val.(type) {
		case string:
			cmd.Flags().String(name, "", "")
			if v != "" {
				_ = cmd.Flags().Set(name, v)
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

// itoa 避免在测试中导入 strconv（其它测试已用 strconv，但这里保持局部零依赖）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// TestBuildClient_DefaultsToNoError 验证 buildClient 在没有任何 flag 的情况下
// 不返回错误（与 buildBizClient 不同——它会因缺 token 报错）。
func TestBuildClient_DefaultsToNoError(t *testing.T) {
	// 清理可能干扰的环境变量
	_ = os.Unsetenv("NAZHI_SSO_BASE")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "",
		"base-url": "",
		"timeout":  0,
	})

	c, err := buildClient(cmd, "base", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 应在无 token 场景下不报错，实际: %v", err)
	}
	if c == nil {
		t.Fatal("buildClient 返回 nil client")
	}
}

// TestBuildClient_HonorsBaseURLFlag 验证 buildClient 接受 base-url flag 而不报错。
// 不验证 client 内部 baseURL（client 是 opaque pointer），只验证不返回 error。
func TestBuildClient_HonorsBaseURLFlag(t *testing.T) {
	_ = os.Unsetenv("NAZHI_SSO_BASE")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "",
		"base-url": "http://example.com:8280",
		"timeout":  0,
	})

	c, err := buildClient(cmd, "base", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 应接受 base-url flag: %v", err)
	}
	if c == nil {
		t.Fatal("buildClient 返回 nil client")
	}
}

// TestBuildClient_HonorsSSOBaseFlag 验证 buildClient 接受 sso-base flag。
func TestBuildClient_HonorsSSOBaseFlag(t *testing.T) {
	_ = os.Unsetenv("NAZHI_SSO_BASE")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "https://sso.example.com",
		"base-url": "",
		"timeout":  0,
	})

	c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 应接受 sso-base flag: %v", err)
	}
	if c == nil {
		t.Fatal("buildClient 返回 nil client")
	}
}

// TestBuildClient_EnvFallback 验证环境变量 NAZHI_SSO_BASE 仍可作为 fallback。
func TestBuildClient_EnvFallback(t *testing.T) {
	t.Setenv("NAZHI_SSO_BASE", "https://env.example.com")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "",
		"base-url": "",
		"timeout":  0,
	})

	c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 应在 env 存在时不报错: %v", err)
	}
	if c == nil {
		t.Fatal("buildClient 返回 nil client")
	}
}

// TestBuildBizClient_StillRequiresToken 回归测试：拆分后 buildBizClient
// 仍必须对 token 必填校验。目的是防止重构后无意中放宽契约。
func TestBuildBizClient_StillRequiresToken(t *testing.T) {
	_ = os.Unsetenv("NAZHI_TOKEN")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"token":    "",
		"base-url": "",
		"timeout":  0,
	})

	_, _, err := buildBizClient(cmd)
	if err == nil {
		t.Fatal("buildBizClient 缺 token 应报错")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("错误信息应提及 token，实际: %v", err)
	}
}

// TestBuildBizClient_HappyPath 验证 buildBizClient 在 token 提供时不报错。
func TestBuildBizClient_HappyPath(t *testing.T) {
	_ = os.Unsetenv("NAZHI_TOKEN")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"token":    "test-token-abc",
		"base-url": "",
		"timeout":  0,
	})

	c, tok, err := buildBizClient(cmd)
	if err != nil {
		t.Fatalf("buildBizClient happy path 失败: %v", err)
	}
	if c == nil {
		t.Fatal("buildBizClient 返回 nil client")
	}
	if tok != "test-token-abc" {
		t.Errorf("token = %q, 期望 %q", tok, "test-token-abc")
	}
}

// TestBuildClient_UploadURLType 验证 urlType="upload" 路径（file_upload 命令用）。
// C2 回归测试：确保 --upload-url flag + NAZHI_UPLOAD_URL env 被正确处理。
func TestBuildClient_UploadURLType(t *testing.T) {
	_ = os.Unsetenv("NAZHI_UPLOAD_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"upload-url": "http://upload.example.com",
		"timeout":    0,
	})

	c, err := buildClient(cmd, "upload", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 应接受 upload-url flag: %v", err)
	}
	if c == nil {
		t.Fatal("buildClient 返回 nil client")
	}
}

// TestBuildClient_UploadURLEnvFallback 验证 urlType="upload" 走 NAZHI_UPLOAD_URL env。
func TestBuildClient_UploadURLEnvFallback(t *testing.T) {
	t.Setenv("NAZHI_UPLOAD_URL", "http://env-upload.example.com")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"upload-url": "",
		"timeout":    0,
	})

	c, err := buildClient(cmd, "upload", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 应在 upload-url env 存在时不报错: %v", err)
	}
	if c == nil {
		t.Fatal("buildClient 返回 nil client")
	}
}

// TestBuildClient_UnknownURLTypeRejected 验证未知 urlType 被拒绝（fail-fast）。
// 防止"调用方写错 urlType 时悄无声息丢 URL"。
func TestBuildClient_UnknownURLTypeRejected(t *testing.T) {
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "",
		"base-url": "",
		"timeout":  0,
	})

	_, err := buildClient(cmd, "bogus", "NAZHI_TIMEOUT")
	if err == nil {
		t.Fatal("buildClient 收到未知 urlType 应报错")
	}
	if !strings.Contains(err.Error(), "urlType") {
		t.Errorf("错误信息应提及 urlType，实际: %v", err)
	}
}

// TestBuildClient_TrackedInPendingClients C3 回归测试：buildClient 构造的 Client
// 必须自动注册到 pendingClients（main 退出前 defer closeAllClients 会释放）。
// 这是 C1+C2 修复的核心目的——消除 inline client.New 让 trackClient 路径统一。
func TestBuildClient_TrackedInPendingClients(t *testing.T) {
	_ = os.Unsetenv("NAZHI_SSO_BASE")
	_ = os.Unsetenv("NAZHI_BASE_URL")
	_ = os.Unsetenv("NAZHI_TIMEOUT")

	// 记录测试前的 baseline
	pendingClientsMu.Lock()
	baseline := len(pendingClients)
	pendingClientsMu.Unlock()

	cmd := makeTestCmdWithFlags(t, map[string]any{
		"sso-base": "",
		"base-url": "",
		"timeout":  0,
	})

	c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
	if err != nil {
		t.Fatalf("buildClient 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close()
		// 清掉这次注册避免污染其他测试
		pendingClientsMu.Lock()
		pendingClients = pendingClients[:baseline]
		pendingClientsMu.Unlock()
	})

	pendingClientsMu.Lock()
	after := len(pendingClients)
	pendingClientsMu.Unlock()

	if after != baseline+1 {
		t.Errorf("buildClient 后 pendingClients 应 +1，实际 baseline=%d after=%d", baseline, after)
	}
}
