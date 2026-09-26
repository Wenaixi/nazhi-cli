package main

// 本文件锁定 CLI 写操作族（honor add / typical-case submit / user update）
// 收敛到共享 runner 后的命令级契约。
//
// 架构深化（C2）：这三个命令与 task submit/edit/preview 共享同一 6 步骨架，
// 但此前各自内联实现（honor add 与 typical-case submit 同形，user update
// 用独立 unknownUserUpdateKeys）。收敛到 runWriteOp + writeOpMode 后，
// 以下行为必须保持不变（用户可见契约）：
//
//  1. 错误优先次序：缺 --payload → 建客户端失败 → 坏 payload → 未知键
//  2. 未知键 / 坏 payload / 缺 payload 不发任何业务请求
//  3. 未知键走参数错误（400/exit3），错误文案含「未知键」与允许键提示
//  4. 成功走 envelope.Empty（无负载，HTTP 204）
//
// 行为变化（由本文件锁定）：
//   - 重构前 typical-case submit 是「先解码后拒未知键」；重构后统一为
//     「先拒未知键后解码」（与 honor add 一致）。合法 payload 下无可见差异，
//     非法 payload 下错误优先次序变化（未知键首报，而非解码错误首报）。
//   - 重构前 user update 的 unknownUserUpdateKeys 大小写敏感；重构后
//     ToLower 折叠（已由 unknown_keys_converged_test.go 锁定）。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// assertOpUnknownKeyRejectsNoRequests 断言命令在含未知键 payload 下
// 不发请求 + 400/exit3 + 「未知键」提示（与 task 族同契约）。
func assertOpUnknownKeyRejectsNoRequests(t *testing.T, name string, cmd *cobra.Command, payload string) {
	t.Helper()
	var hit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":500,"msg":"不应到达"}`))
	}))
	defer server.Close()

	testCmd := &cobra.Command{Use: name}
	testCmd.SetContext(context.Background())
	testCmd.Flags().String("token", "", "")
	_ = testCmd.Flags().Set("token", "test-token")
	testCmd.Flags().String("base-url", "", "")
	_ = testCmd.Flags().Set("base-url", server.URL)
	testCmd.Flags().Int("timeout", 5, "")
	testCmd.Flags().String("payload", "", "")
	_ = testCmd.Flags().Set("payload", payload)

	quiet, verbose = false, false
	pendingExitCode.Store(0)
	stdout, stderr, restore := captureStdio(t)
	cmd.Run(testCmd, nil)
	restore()

	if hit {
		t.Fatalf("%s: 含未知键时不应发出任何请求", name)
	}
	if pendingExitCode.Load() != 3 {
		t.Errorf("%s: 未知键应走参数错误退出码 3，实际 %d", name, pendingExitCode.Load())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, `"code": 400`) {
		t.Errorf("%s: 应输出 400 envelope，实际 stdout=%s stderr=%s", name, stdout.String(), stderr.String())
	}
	if !strings.Contains(combined, "未知键") {
		t.Errorf("%s: 应含未知键提示，实际 stdout=%s stderr=%s", name, stdout.String(), stderr.String())
	}
}

// TestHonorAdd_UnknownKey_NoRequests 锁定 honor add 未知键拒绝 + 零请求。
func TestHonorAdd_UnknownKey_NoRequests(t *testing.T) {
	assertOpUnknownKeyRejectsNoRequests(t, "honor add", honorAddCmd, `{"typeId":1,"level":5,"typeNameX":"错拼"}`)
}

// TestTypicalCaseSubmit_UnknownKey_NoRequests 锁定 typical-case submit 未知键拒绝 + 零请求。
func TestTypicalCaseSubmit_UnknownKey_NoRequests(t *testing.T) {
	assertOpUnknownKeyRejectsNoRequests(t, "typical-case submit", typicalCaseSubmitCmd, `{"title":"x","content":"y","titelX":"错拼"}`)
}

// TestUserUpdate_UnknownKey_NoRequests 锁定 user update 未知键拒绝 + 零请求。
func TestUserUpdate_UnknownKey_NoRequests(t *testing.T) {
	assertOpUnknownKeyRejectsNoRequests(t, "user update", userUpdateCmd, `{"telephone":"138","telephoneX":"错拼"}`)
}

// TestWriteOpFamily_MissingPayloadPrecedence 锁定四个写操作命令
// （honor add / typical submit / user update / task submit）缺 payload
// 优先于缺 token。
func TestWriteOpFamily_MissingPayloadPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"task submit", taskSubmitCmd},
		{"honor add", honorAddCmd},
		{"typical-case submit", typicalCaseSubmitCmd},
		{"user update", userUpdateCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "write-op"}
			cmd.SetContext(context.Background())
			cmd.Flags().String("token", "", "")
			cmd.Flags().String("base-url", "", "")
			cmd.Flags().Int("timeout", 5, "")
			cmd.Flags().String("payload", "", "")

			quiet, verbose = false, false
			pendingExitCode.Store(0)
			_, stderr, restore := captureStdio(t)
			tc.cmd.Run(cmd, nil)
			restore()

			if !strings.Contains(stderr.String(), "--payload 为必填") {
				t.Errorf("%s: 缺 payload 应优先于缺 token，实际: %q", tc.name, stderr.String())
			}
		})
	}
}
