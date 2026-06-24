package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// unifiedOKEmpty 模拟业务 API 返回 code=1 但 returnData/dataMap 都为 nil 的响应。
// SDK 文档明确：GetMyInfo "最佳努力设计：失败返回 nil，不中断主流程"。
// 此时 cmd 层不应打印 error。
const unifiedOKEmpty = `{"code":1,"msg":"成功"}`

// makeWhoamiTestCmd 构造 whoami 命令的测试用 cobra.Command，
// 通过 cmd.SetContext 注入一个带 mock server 的 bizURL。
func makeWhoamiTestCmd(t *testing.T, token string) (*cobra.Command, *client.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session 预热路径必须先响应
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			// 业务响应：code=1 但 returnData/dataMap 都为 nil → SDK 返回 (nil, nil)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(unifiedOKEmpty))
		default:
			w.Header().Set("Content-Type", "application/json")
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

	cmd := &cobra.Command{Use: "whoami"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", token, "")
	cmd.Flags().String("base-url", srv.URL, "")
	cmd.Flags().Int("timeout", 5, "")
	return cmd, c
}

// captureStdio 替换 os.Stdout/os.Stderr 并返回还原函数。
// 调完测试逻辑后 defer restore() 即可恢复。
func captureStdio(t *testing.T) (stdout *bytes.Buffer, stderr *bytes.Buffer, restore func()) {
	t.Helper()
	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) 失败: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) 失败: %v", err)
	}
	os.Stdout, os.Stderr = wOut, wErr

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	restore = func() {
		_ = wOut.Close()
		_ = wErr.Close()
		go func() { _, _ = io.Copy(stdoutBuf, rOut) }()
		go func() { _, _ = io.Copy(stderrBuf, rErr) }()
		os.Stdout, os.Stderr = origStdout, origStderr
	}
	return stdoutBuf, stderrBuf, restore
}

// TestWhoami_GetMyInfoReturnsNil_NotTreatedAsError 回归测试：GetMyInfo 返回
// (nil, nil)（HTTP 200 + code=1 + returnData/dataMap 都为 nil）时，
// whoami 命令必须输出 JSON（null），**不**打印错误并走 os.Exit 路径。
//
// 历史 bug（F5）：whoami.go:31 把 (nil, nil) 误当成 fatal error，调
// printError("未找到用户信息")，违反 SDK "最佳努力设计" 契约。
func TestWhoami_GetMyInfoReturnsNil_NotTreatedAsError(t *testing.T) {
	cmd, _ := makeWhoamiTestCmd(t, "test-token")

	// 抑制 quiet 防止 printError 吞 stderr
	quiet = false
	pendingExitCode.Store(0)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	origStdout, origStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr
	defer func() {
		os.Stdout, os.Stderr = origStdout, origStderr
	}()
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(stdoutBuf, rOut)
		copyDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(stderrBuf, rErr)
		copyDone <- struct{}{}
	}()

	// 关键：调 Run 回调（不能直接 Execute，否则 init() 注册的所有子命令都会被触发）
	whoamiCmd.Run(cmd, nil)

	// 关闭 writer 等 reader 读完
	_ = wOut.Close()
	_ = wErr.Close()
	<-copyDone
	<-copyDone

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码必须保持 0（不是 error 路径）
	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("GetMyInfo 返回 (nil, nil) 不应触发 pendingExitCode=1，实际 %d", got)
	}

	// 关键断言 1：stderr 不应包含 "未找到用户信息"（修复前会）
	if strings.Contains(stderr, "未找到用户信息") {
		t.Errorf("stderr 不应包含 '未找到用户信息'（F5: SDK nil 不算 error），实际: %q", stderr)
	}
	// 关键断言 2：stderr 不应包含 error JSON 标记
	if strings.Contains(stderr, `"error": true`) {
		t.Errorf("stderr 不应包含 error JSON，实际: %q", stderr)
	}

	// 关键断言 3：stdout 应输出 nil JSON（printJSON(nil) → "null\n"）
	// 允许两种合理输出："null" 或 "null\n"（JSON encoder 会加换行）
	if !strings.Contains(stdout, "null") {
		t.Errorf("stdout 应包含 'null'（printJSON(nil) 输出），实际: %q", stdout)
	}
}
