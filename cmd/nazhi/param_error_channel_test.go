package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// 这组测试锁定「参数错误只有一条输出通道」这条用户可见契约。
//
// 背景：参数错误（400 / 退出码 3）此前被劈成两个通道——printParamError 与
// printErrorWithCode 走 stderr 共 47 处，printEnvelope(envelope.Error(400))
// 走 stdout 共 29 处。规则不存在于任何可学的地方：同一个写操作命令的 6 步
// 控制流里第 1 步（缺 payload）与第 6 步（id 校验）走 stdout、中间 4 步
// 走 stderr；honor list 的分页守卫走 stdout 而 circle images 的同类守卫走
// stderr。测试还刻意用 stdout+stderr 合并断言，把不一致固化成了「不必回答」。
//
// output.go 早已声明「参数错误请用 printParamError（400→exit3）」，项目
// 记忆库也记着「成功输出 stdout，错误写 stderr」——那 29 处是违反自家
// 契约的偏离。调用方（脚本作者、AI 代理）无法从任何地方学到该去哪读错误。
//
// 本组测试断言 printParamError 这一个出口的通道，而不是逐命令重复断言：
// 逐命令断言只能覆盖已写下的命令，对新增命令无效。生产侧靠「参数错误只经
// printParamError 出口」这条纪律收敛（见 CLAUDE.md「D. CLI 契约」）。

// callPrintParamError 在捕获下执行一次 printParamError，返回两通道内容与退出码。
func callPrintParamError(t *testing.T, err error, quietMode bool) (string, string, int32) {
	t.Helper()
	originalQuiet := quiet
	quiet = quietMode
	pendingExitCode.Store(0)
	stdout, stderr, restore := captureStdio(t)
	printParamError(err)
	restore()
	quiet = originalQuiet
	code := pendingExitCode.Load()
	pendingExitCode.Store(0)
	return stdout.String(), stderr.String(), code
}

// callPrintError 在捕获下执行一次 printError，返回两通道内容。
func callPrintError(t *testing.T, err error) (string, string) {
	t.Helper()
	pendingExitCode.Store(0)
	stdout, stderr, restore := captureStdio(t)
	printError(err)
	restore()
	code := pendingExitCode.Load()
	pendingExitCode.Store(0)
	_ = code
	return stdout.String(), stderr.String()
}

// TestParamErrorChannelIsUniform 参数错误必须只写 stderr，stdout 保持干净。
func TestParamErrorChannelIsUniform(t *testing.T) {
	stdout, stderr, code := callPrintParamError(t, errors.New("缺 token"), false)
	if !strings.Contains(stderr, "缺 token") {
		t.Fatalf("参数错误信封未写入 stderr：stderr=%q", stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("参数错误污染了 stdout 通道：stdout=%q", stdout)
	}
	if code != 3 {
		t.Errorf("参数错误退出码 = %d，期望 3", code)
	}
}

// TestSuccessStillGoesToStdout 统一错误通道不得波及成功路径：
// stdout 仍是脚本读取数据的唯一通道。
func TestSuccessStillGoesToStdout(t *testing.T) {
	stdout, stderr, restore := captureStdio(t)
	printEnvelope(envelope.Success(map[string]string{"k": "v"}))
	restore()
	if strings.TrimSpace(stdout.String()) == "" {
		t.Errorf("成功响应未写入 stdout")
	}
	if strings.TrimSpace(stderr.String()) != "" {
		t.Errorf("成功响应污染了 stderr：stderr=%q", stderr.String())
	}
}

// TestParamErrorAndBusinessErrorShareChannel 参数错误与业务错误走同一通道，
// 但信封 code 不同（400 vs 500）。脚本因此只需学一条规则：
// 退出码非 0 时读 stderr，退出码与 code 区分错误类别。
func TestParamErrorAndBusinessErrorShareChannel(t *testing.T) {
	_, paramStderr, _ := callPrintParamError(t, errors.New("参数"), false)
	_, bizStderr := callPrintError(t, errors.New("业务"))

	if strings.TrimSpace(paramStderr) == "" || strings.TrimSpace(bizStderr) == "" {
		t.Fatalf("两类错误都必须写 stderr：param=%q biz=%q", paramStderr, bizStderr)
	}
	if !strings.Contains(paramStderr, `"code": 400`) {
		t.Errorf("参数错误信封 code 不是 400：%s", paramStderr)
	}
	if !strings.Contains(bizStderr, `"code": 500`) {
		t.Errorf("业务错误信封 code 不是 500：%s", bizStderr)
	}
}

// TestLoginErrorChannels 登录的三类失败（限流 429 / 服务端不可用 502 /
// 凭据被拒 401）必须与其它错误同走 stderr，只靠退出码与 code 区分。
//
// 这三条路径此前用 printEnvelope 写 stdout，而同一 switch 的 default 分支
// 走 printError（stderr）——同一命令内两条通道，本组测试防止其复发。
// 用例注入假客户端避免真实网络：登录失败分支由 SDK 错误链驱动。
func TestLoginErrorChannels(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode string
		wantText string
	}{
		{"限流", http.StatusTooManyRequests, `{"code":-1,"msg":"rate limited"}`, "429", "请求被限流"},
		{"服务端不可用", http.StatusBadGateway, `{"code":-1,"msg":"bad gateway"}`, "502", "暂时不可用"},
		// 200 但响应里没有 token：SDK 归 ErrLoginRejected（auth.go:215），
		// 走 login.go 的 401 分支。此前该分支无任何通道断言。
		{"凭据被拒", http.StatusOK, `{"code":-1,"msg":"账号或密码错误"}`, "401", "请检查学号/密码"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := mockLoginSSO(t, tc.status, tc.body)
			originalQuiet := quiet
			quiet = false
			pendingExitCode.Store(0)
			stdout, stderr, restore := captureStdio(t)
			loginCmd.Run(newLoginTestCmd(srv.URL), nil)
			restore()
			quiet = originalQuiet
			code := pendingExitCode.Load()
			pendingExitCode.Store(0)
			if strings.TrimSpace(stdout.String()) != "" {
				t.Errorf("登录失败不应写 stdout，实际: %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), `"code": `+tc.wantCode) {
				t.Errorf("stderr 未含 code %s：%s", tc.wantCode, stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantText) {
				t.Errorf("stderr 未含专属文案 %q：%s", tc.wantText, stderr.String())
			}
			if code == 0 {
				t.Errorf("登录失败必须设置非零退出码")
			}
		})
	}
}

// TestLoginParamErrorGoesToStderr 缺 username/password 属参数错误，
// 同样写 stderr 且退出码为 3。
func TestLoginParamErrorGoesToStderr(t *testing.T) {
	for _, tc := range []struct{ name, user, pass string }{
		{"username 空白", "   ", "secret"},
		{"password 空白", "student001", " \t "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "login"}
			cmd.SetContext(context.Background())
			cmd.Flags().String("username", tc.user, "")
			cmd.Flags().String("password", tc.pass, "")
			cmd.Flags().String("sso-base", "", "")
			cmd.Flags().Int("timeout", 15, "")
			originalQuiet := quiet
			quiet = false
			pendingExitCode.Store(0)
			stdout, stderr, restore := captureStdio(t)
			loginCmd.Run(cmd, nil)
			restore()
			quiet = originalQuiet
			code := pendingExitCode.Load()
			pendingExitCode.Store(0)
			if strings.TrimSpace(stdout.String()) != "" {
				t.Errorf("登录参数错误不应写 stdout，实际: %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), `"code": 400`) {
				t.Errorf("stderr 未含 400 信封：%s", stderr.String())
			}
			if code != 3 {
				t.Errorf("退出码 = %d，期望 3", code)
			}
		})
	}
}

// TestQuietModeStillSetsExitCodeWithoutWriting quiet 模式下不写任何通道，
// 但退出码必须照常设置——否则脚本会把参数错误误判为成功。
func TestQuietModeStillSetsExitCodeWithoutWriting(t *testing.T) {
	stdout, stderr, code := callPrintParamError(t, errors.New("缺 token"), true)
	if code != 3 {
		t.Errorf("quiet 模式下参数错误退出码 = %d，期望 3", code)
	}
	if strings.TrimSpace(stderr) != "" || strings.TrimSpace(stdout) != "" {
		t.Errorf("quiet 模式不应写任何通道：stdout=%q stderr=%q", stdout, stderr)
	}
}

// TestParamErrorIsRedacted 参数错误的信封消息必须经脱敏：
// 底层错误链里的形似学号标识符不得进入用户可见输出。
func TestParamErrorIsRedacted(t *testing.T) {
	// 使用与 PII 守卫同形的占位值：形似即拦，不写任何真实标识符
	leaky := "参数错误 token=abcdef123456 userName=G350181200912110035"
	_, stderr, _ := callPrintParamError(t, errors.New(leaky), false)
	if strings.Contains(stderr, "G350181200912110035") {
		t.Errorf("参数错误信封泄漏了形似学号的标识符：%s", stderr)
	}
}
