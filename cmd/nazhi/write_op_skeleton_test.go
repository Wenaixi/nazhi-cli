package main

// 本文件锁定 CLI 写操作族（task submit/edit/preview）共享控制流的收敛后契约。
//
// 架构深化（C2）：task submit/edit/preview 三个命令此前各内联一份 6 步骨架
// （读 payload → 判空 → 建客户端 → 解析 → 拒未知键 → 解码 → 覆盖 flag → 调用 →
// envelope），submit 与 edit 33 行逐字重复、preview 内部分叉两份。收敛为
// 共享 runner 后，以下命令级行为必须保持不变（这些是用户可见契约）：
//
//  1. 错误优先次序：缺 --payload → 建客户端失败 → 坏 payload（非对象）→ 未知键
//  2. 未知键 / 坏 payload / 缺 payload 均不发任何业务请求（含元数据预热）
//  3. 未知键走参数错误（400/exit3），错误文案含「未知键」与允许键提示
//  4. --address/--level flag 覆盖 payload 中的同名值
//  5. preview --edit 走编辑分支（decodeTaskEditInput），否则提交分支
//
// 这些测试在收敛前后都必须通过（绿）——它们防止收敛悄悄改变命令行为。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// makeWriteOpTestCmd 构造一个带通用业务参数 + payload 的测试命令实例。
// 与三个命令的真实 flag 形状对齐（token/base-url/timeout/payload/address/level/edit）。
func makeWriteOpTestCmd(t *testing.T, baseURL, payload string, extra map[string]string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "write-op-test"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().String("payload", "", "")
	if payload != "" {
		_ = cmd.Flags().Set("payload", payload)
	}
	cmd.Flags().String("address", "", "")
	cmd.Flags().String("level", "", "")
	cmd.Flags().Bool("edit", false, "")
	for k, v := range extra {
		_ = cmd.Flags().Set(k, v)
	}
	return cmd
}

// assertUnknownKeyRejectsNoRequests 断言命令在含未知键 payload 下：
//   - 不发任何请求（server 命中即失败）
//   - 退出码 3（参数错误）
func assertUnknownKeyRejectsNoRequests(t *testing.T, name string, run func(*cobra.Command, []string)) {
	var hit bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":500,"msg":"不应到达"}`))
	}))
	defer server.Close()

	cmd := makeWriteOpTestCmd(t, server.URL, `{"taskId":1,"content":"x","imagePath":"./a.jpg"}`, nil)
	quiet, verbose = false, false
	pendingExitCode.Store(0)
	stdout, stderr, restore := captureStdio(t)
	run(cmd, nil)
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

// TestTaskSubmit_UnknownKey_NoRequests 锁定 task submit 未知键拒绝 + 零请求。
func TestTaskSubmit_UnknownKey_NoRequests(t *testing.T) {
	assertUnknownKeyRejectsNoRequests(t, "task submit", taskSubmitCmd.Run)
}

// TestTaskEdit_UnknownKey_NoRequests 锁定 task edit 未知键拒绝 + 零请求。
func TestTaskEdit_UnknownKey_NoRequests(t *testing.T) {
	assertUnknownKeyRejectsNoRequests(t, "task edit", taskEditCmd.Run)
}

// TestTaskPreview_UnknownKey_NoRequests 锁定 task preview 未知键拒绝 + 零请求。
func TestTaskPreview_UnknownKey_NoRequests(t *testing.T) {
	assertUnknownKeyRejectsNoRequests(t, "task preview", taskPreviewCmd.Run)
}

// TestWriteOp_MissingPayloadPrecedence 锁定三命令缺 --payload 优先于缺 token。
func TestWriteOp_MissingPayloadPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"task submit", taskSubmitCmd},
		{"task edit", taskEditCmd},
		{"task preview", taskPreviewCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "write-op"}
			cmd.SetContext(context.Background())
			cmd.Flags().String("token", "", "")
			cmd.Flags().String("base-url", "", "")
			cmd.Flags().Int("timeout", 5, "")
			cmd.Flags().String("payload", "", "")
			cmd.Flags().String("address", "", "")
			cmd.Flags().String("level", "", "")
			cmd.Flags().Bool("edit", false, "")

			quiet, verbose = false, false
			pendingExitCode.Store(0)
			_, stderr, restore := captureStdio(t)
			tc.cmd.Run(cmd, nil)
			restore()

			if !strings.Contains(stderr.String(), "--payload 为必填") {
				t.Errorf("%s: 缺 payload 应优先于缺 token 报 --payload 为必填，实际: %q", tc.name, stderr.String())
			}
		})
	}
}

// TestWriteOp_AddressLevelFlagOverrideLogic 锁定 --address/--level 覆盖 payload 值
// 的语义（经 applyAddressLevelFlags 真实现）：非空 flag 覆盖、空 flag 保留
// payload 原值。不依赖网络（flag 覆盖发生在建客户端之后、调用 SDK 之前，
// 是纯内存操作）。
func TestWriteOp_AddressLevelFlagOverrideLogic(t *testing.T) {
	t.Run("非空 flag 覆盖 payload 值", func(t *testing.T) {
		input := types.TaskSubmitInput{Address: "payload地址", Level: ""}
		cmd := &cobra.Command{}
		cmd.Flags().String("address", "", "")
		cmd.Flags().String("level", "", "")
		_ = cmd.Flags().Set("address", "flag地址")
		_ = cmd.Flags().Set("level", "5")
		applyAddressLevelFlags(cmd, func(address, level string) {
			if address != "" {
				input.Address = address
			}
			if level != "" {
				input.Level = level
			}
		})
		if input.Address != "flag地址" || input.Level != "5" {
			t.Fatalf("非空 flag 应覆盖: got address=%q level=%q", input.Address, input.Level)
		}
	})
	t.Run("空 flag 保留 payload 原值", func(t *testing.T) {
		input := types.TaskSubmitInput{Address: "payload地址", Level: "3"}
		cmd := &cobra.Command{}
		cmd.Flags().String("address", "", "")
		cmd.Flags().String("level", "", "")
		applyAddressLevelFlags(cmd, func(address, level string) {
			if address != "" {
				input.Address = address
			}
			if level != "" {
				input.Level = level
			}
		})
		if input.Address != "payload地址" || input.Level != "3" {
			t.Fatalf("空 flag 应保留 payload 值: got address=%q level=%q", input.Address, input.Level)
		}
	})
}

// TestWriteOp_ClientBuildFailurePrecedesBadPayload 锁定 runWriteOp 中
// 「建客户端失败」与「坏 payload」的相对次序：先建客户端（write_op_runner.go
// 的第 2 步），再解析 payload。
//
// 该次序此前只有注释描述、无任何测试锁定，注释一度写成「坏 payload 在前」
// 而实现是反的，长期无人发现。此处用「base-url 不可解析 + payload 非法」
// 的双缺陷输入固定次序：无论先判哪个，只会报其中一个，据此断言是哪一个。
func TestWriteOp_ClientBuildFailurePrecedesBadPayload(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{"task submit", taskSubmitCmd},
		{"task edit", taskEditCmd},
		{"task preview", taskPreviewCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "write-op"}
			cmd.SetContext(context.Background())
			cmd.Flags().String("token", "", "")
			_ = cmd.Flags().Set("token", "test-token")
			cmd.Flags().String("base-url", "", "")
			// 含控制字符的 URL 会让 url.Parse 失败，建客户端必然报错。
			_ = cmd.Flags().Set("base-url", "http://example.com\x7f with space")
			cmd.Flags().Int("timeout", 5, "")
			cmd.Flags().String("payload", "", "")
			// 同时给出非法 payload：非 JSON 文本。两条路径都会失败。
			_ = cmd.Flags().Set("payload", "not-json-at-all")
			cmd.Flags().String("address", "", "")
			cmd.Flags().String("level", "", "")
			cmd.Flags().Bool("edit", false, "")

			originalQuiet, originalVerbose := quiet, verbose
			quiet, verbose = false, false
			pendingExitCode.Store(0)
			t.Cleanup(func() {
				quiet, verbose = originalQuiet, originalVerbose
				pendingExitCode.Store(0)
				_ = closeAllClients()
			})

			_, stderr, restore := captureStdio(t)
			tc.cmd.Run(cmd, nil)
			restore()

			if strings.Contains(stderr.String(), "读取 payload 失败") {
				t.Errorf("%s: 实现先建客户端后解析 payload，坏 payload 不应先报；实际 stderr=%q", tc.name, stderr.String())
			}
			if !strings.Contains(stderr.String(), "error") {
				t.Errorf("%s: 双缺陷输入下应输出错误信封，实际 stderr=%q", tc.name, stderr.String())
			}
		})
	}
}
