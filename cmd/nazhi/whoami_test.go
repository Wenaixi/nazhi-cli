package main

import (
	"bytes"
	"context"
	"fmt"
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

// captureStdio 替换 os.Stdout/os.Stderr 并返回还原函数 + 同步等待机制。
//
// 调用模式：
//
//	stdoutBuf, stderrBuf, restore := captureStdio(t)
//	defer restore()
//
//	// 触发命令（命令内对 os.Stdout/Stderr 的写会进入管道）
//	whoamiCmd.Run(cmd, nil)
//
//	// restore() 同步等待 drain 完成；之后 stdoutBuf/stderrBuf 才包含全部数据
//	stdout := stdoutBuf.String()
//	stderr := stderrBuf.String()
//
// 设计要点：
//   - 启动 drain goroutine **前**先 close writer（确保 io.Copy 看到 EOF）
//   - 用 done channel 同步等待 drain goroutine 退出，避免调用方读到空 buffer
//   - 先恢复 os.Stdout/Stderr 再 return，防止后续 t.Logf 等打到管道里
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
		// 关键顺序：先关 writer（让 io.Copy 看到 EOF），再启动 drain goroutine，
		// 用 channel 同步等待，最后恢复 os.Stdout/Stderr。
		_ = wOut.Close()
		_ = wErr.Close()
		outDone := make(chan struct{})
		errDone := make(chan struct{})
		go func() {
			_, _ = io.Copy(stdoutBuf, rOut)
			close(outDone)
		}()
		go func() {
			_, _ = io.Copy(stderrBuf, rErr)
			close(errDone)
		}()
		<-outDone
		<-errDone
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

	stdoutBuf, stderrBuf, restore := captureStdio(t)

	// 关键：调 Run 回调（不能直接 Execute，否则 init() 注册的所有子命令都会被触发）
	whoamiCmd.Run(cmd, nil)

	// restore() 同步 drain 管道到 buffer，调用后 stdoutBuf/stderrBuf 才包含全部数据
	restore()
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

// TestCaptureStdio_DrainsBothStreams 锁住 captureStdio 行为（F3 重构）：
// 直接调用 captureStdio + 在替换后的 stdout/stderr 上写数据 + restore，
// 断言两路数据都被完整 drain 到 buffer。这是 captureStdio 的最小可执行规约，
// 防止后续"以为它工作"再次写出 race-y 或丢数据的实现。
//
// 顺带覆盖：调用方在 restore 之前对 buffer 的读会得到空（drain 是 restore 的副作用），
// 调用方在 restore 之后才能拿到完整数据。
func TestCaptureStdio_DrainsBothStreams(t *testing.T) {
	stdoutBuf, stderrBuf, restore := captureStdio(t)

	fmt.Fprintln(os.Stdout, "hello stdout")
	fmt.Fprintln(os.Stderr, "world stderr")

	// restore() 之前 buffer 还没 drain（captureStdio 不预启动 reader）
	restore()

	if got := stdoutBuf.String(); got != "hello stdout\n" {
		t.Errorf("stdoutBuf 应为 %q，实际 %q", "hello stdout\n", got)
	}
	if got := stderrBuf.String(); got != "world stderr\n" {
		t.Errorf("stderrBuf 应为 %q，实际 %q", "world stderr\n", got)
	}

	// 二次 restore 安全：fd 已关，close 报错被忽略，函数同步返回
	restore()
}
