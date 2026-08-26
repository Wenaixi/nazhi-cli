package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// makeSessionActivateTestCmd 创建 session activate 命令的测试用 cobra.Command + mock server。
// getMyInfoBody: /api/studentInfo/getMyInfo 的响应体 JSON
//   - 空字符串: 默认 returnData/dataMap 都为 nil（触发空响应路径）
//   - 正常 JSON: returnData 含 name/studentNumber 等字段
func makeSessionActivateTestCmd(t *testing.T, token string, getMyInfoBody string) *cobra.Command {
	t.Helper()
	if getMyInfoBody == "" {
		getMyInfoBody = `{"code":1,"msg":"成功"}`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(getMyInfoBody))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		}
	}))
	t.Cleanup(srv.Close)

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithSSOBase(srv.URL),
		client.WithToken(token),
	)
	trackClient(c)
	t.Cleanup(func() { _ = c.Close() })

	cmd := &cobra.Command{Use: "session-activate"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	// 必须显式 Set 让 Changed()=true，否则 buildClientOpts 走 env fallback。
	if err := cmd.Flags().Set("token", token); err != nil {
		t.Fatalf("set token flag: %v", err)
	}
	cmd.Flags().String("base-url", "", "")
	// 必须显式 Set 让 Changed()=true，否则 buildClientOpts 走 env fallback。
	if err := cmd.Flags().Set("base-url", srv.URL); err != nil {
		t.Fatalf("set base-url flag: %v", err)
	}
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// TestSessionActivate_EmptyUserInfo_StatusEnvelope 回归测试：空 UserInfo 输出 status envelope
// session activate 命令在 ActivateSession 返回 (nil, ErrEmptyUserInfo) 时
// 必须输出 status envelope，**不**输出裸 null。
// 失败场景：cmd/nazhi/session.go:38 printJSON(info) → 输出 `null\n`
// 与 whoami 的 {status: empty, reason: ...} 不一致。
// 修复后：cmd 层用 errors.Is(err, ErrEmptyUserInfo) 分支输出 status envelope。
func TestSessionActivate_EmptyUserInfo_StatusEnvelope(t *testing.T) {
	cmd := makeSessionActivateTestCmd(t, "test-token", "")

	quiet = false
	pendingExitCode.Store(0)

	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// 关键：sessionActivateCmd.Run 是闭包，调 sessionActivateCmd.Run(cmd, nil)
	sessionActivateCmd.Run(cmd, nil)

	_ = wOut.Close()
	_ = wErr.Close()
	outDone := make(chan struct{})
	errDone := make(chan struct{})
	go func() { _, _ = io.Copy(stdoutBuf, rOut); close(outDone) }()
	go func() { _, _ = io.Copy(stderrBuf, rErr); close(errDone) }()
	<-outDone
	<-errDone
	os.Stdout, os.Stderr = origStdout, origStderr

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码应为 2（envelope.Empty 成功路径：code=204 → exit code 0，
	// 但本测试是 ErrEmptyUserInfo 兜底，期望 envelope.Empty() 走 stdout，
	// 而 whoami/session 用 envelope.Empty 时不再 markError（204 是 success）
	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("envelope.Empty 应不触发 markError（status=success），实际 %d", got)
	}

	// stderr 不应有 error 标记
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error JSON（应输出 envelope.Empty），实际: %q", stderr)
	}

	// stdout 应输出 envelope.Empty 而非裸 null
	if strings.TrimSpace(stdout) == "null" {
		t.Errorf("stdout 不应是裸 null（应输出 envelope.Empty），实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"status": "success"`) {
		t.Errorf("stdout 应包含 status=success（envelope.Empty），实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"code": 204`) {
		t.Errorf("stdout 应包含 code=204（envelope.Empty），实际: %q", stdout)
	}
}

// TestSessionActivate_ValidUserInfo_OutputsUserInfo 验证正常响应时
// session activate 直接输出 UserInfo（向后兼容 happy path）。
func TestSessionActivate_ValidUserInfo_OutputsUserInfo(t *testing.T) {
	validBody := `{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`
	cmd := makeSessionActivateTestCmd(t, "test-token", validBody)

	quiet = false
	pendingExitCode.Store(0)

	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	sessionActivateCmd.Run(cmd, nil)

	_ = wOut.Close()
	_ = wErr.Close()
	outDone := make(chan struct{})
	errDone := make(chan struct{})
	go func() { _, _ = io.Copy(stdoutBuf, rOut); close(outDone) }()
	go func() { _, _ = io.Copy(stderrBuf, rErr); close(errDone) }()
	<-outDone
	<-errDone
	os.Stdout, os.Stderr = origStdout, origStderr

	stdout := stdoutBuf.String()

	if !strings.Contains(stdout, `"name": "张三"`) {
		t.Errorf("正常响应 stdout 应包含 name: 张三，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"studentNumber": "TEST2025001"`) {
		t.Errorf("正常响应 stdout 应包含 studentNumber，实际: %q", stdout)
	}
}

// 防止 errors 包未使用（编译静态检查）
var _ = errors.Is

// TestSessionActivate_ErrSessionBackoff_CooldownMessage 回归测试 backoff 冷却提示
// session activate 命令在 ActivateSession 返回 ErrSessionBackoff 时
// 必须输出 friendly cooldown 提示（不输出 error JSON / 不标记退出码 1）。
// 历史问题：ErrSessionBackoff 哨兵在 cmd/nazhi 层零消费
// 直接走 printError 输出 {"error":true,"message":"...backoff..."}。
// 用户看到一个 JSON 错误，不知道是"需要等待"还是"真的出错了"。
// 测试策略：手动构建带 backoff 状态的 Client，验证 cmd 层对
// ErrSessionBackoff 的输出格式。
func TestSessionActivate_ErrSessionBackoff_CooldownMessage(t *testing.T) {
	// 创建模拟服务器，getMyInfo 始终失败以触发 backoff
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		}
	}))
	defer srv.Close()

	// 直接构建 Client，不通过 cobra 命令（因为 sessionActivateCmd.Run 内部调
	// buildBizClient 新建 Client，无法保留 backoff 状态）
	c, err := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("构建 Client 失败: %v", err)
	}
	defer func() { _ = c.Close() }()

	// 第一次调用：失败（设置 backoff）
	if _, err := c.ActivateSession(context.Background(), "test-token"); err == nil {
		t.Fatal("第一次激活应失败（getMyInfo 返回 500）")
	}

	// 第二次调用：命中 backoff，返回 ErrSessionBackoff
	_, backoffErr := c.ActivateSession(context.Background(), "test-token")
	if !errors.Is(backoffErr, client.ErrSessionBackoff) {
		t.Fatalf("第二次激活应命中 backoff，实际: %v", backoffErr)
	}

	// 捕获 stdout/stderr
	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)

	// 模拟 cmd 层对 ErrSessionBackoff 的处理逻辑（与 sessionActivateCmd.Run
	// 中的 if 分支相同逻辑）。这里**手动模拟** cmd 处理路径：
	// sessionActivateCmd.Run 因 buildBizClient 每次新建 Client，无法保留 backoff
	// 状态，无法通过 cobra 命令路径触发 backoff 分支；本测试改为单元级合约测试：
	// 当 cmd 层处理 ErrSessionBackoff 时，必须同时设 pendingExitCode=1（partial envelope）。
	if errors.Is(backoffErr, client.ErrSessionBackoff) {
		printEnvelope(envelope.Partial(429, "session 激活冷却中，上次激活失败请稍后重试", nil))
	} else {
		printError(backoffErr)
	}

	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码应为 1（envelope.Partial(429) → exit code 1）
	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("backoff 应触发 pendingExitCode=1（envelope.Partial(429)），实际 %d", got)
	}

	// stderr 不应包含 error 标记
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}

	// stdout 应输出 partial 状态
	if !strings.Contains(stdout, `"status": "partial"`) {
		t.Errorf("stdout 应包含 status: partial，实际: %q", stdout)
	}

	// stdout 应包含友好提示（请稍后重试 / 冷却等）
	if !strings.Contains(stdout, "冷却") {
		t.Errorf("stdout 应包含冷却提示，实际: %q", stdout)
	}
}

// TestSessionCmd_ActivateRegisteredOnce 回归：activate 子命令只能注册一次。
// cobra AddCommand 不去重（直接 append），历史双注册（main.go 与 session.go init
// 各注册一次同一指针）让 nazhi session --help 出现两行相同的 activate 条目。
func TestSessionCmd_ActivateRegisteredOnce(t *testing.T) {
	count := 0
	for _, sub := range sessionCmd.Commands() {
		if sub.Name() == "activate" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("session activate 应恰好注册 1 次，实际 %d 次", count)
	}
}
