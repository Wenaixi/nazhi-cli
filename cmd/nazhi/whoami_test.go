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
// 此时 cmd 层不应打印 error，应输出 {"status":"empty","reason":"get_my_info_empty"}。
const unifiedOKEmpty = `{"code":1,"msg":"成功"}`

// unifiedUserInfo 模拟 getMyInfo 返回完整用户信息的响应。
const unifiedUserInfo = `{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","schoolName":"福清一中","className":"高一八班","seat":45}}`

// makeWhoamiTestCmd 创建 whoami 命令的测试用 cobra.Command + mock server。
// getMyInfoBody 是 /api/studentInfo/getMyInfo 的响应体 JSON，空字符串时默认 unifiedOKEmpty。
// bizOK 控制 session 预热路径（首页+/getMenu）是否返回 200 OK（false 则返回 500 模拟激活失败）。
func makeWhoamiTestCmd(t *testing.T, token string, getMyInfoBody string, bizOK ...bool) *cobra.Command {
	t.Helper()
	if getMyInfoBody == "" {
		getMyInfoBody = unifiedOKEmpty
	}
	ok := true
	if len(bizOK) > 0 {
		ok = bizOK[0]
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// session 预热路径必须先响应
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			if !ok {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			// 业务响应由调用方控制
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(getMyInfoBody))
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
	cmd.Flags().String("token", "", "")
	// F7 适配：buildClientOpts 用 flagChanged() 守卫 token 读取。
	// 必须调 Set 才能让 Changed()=true，否则走 env fallback 会因 env 未设而报缺 token。
	if err := cmd.Flags().Set("token", token); err != nil {
		t.Fatalf("set token flag: %v", err)
	}
	cmd.Flags().String("base-url", "", "")
	// B12 适配：必须 Set 让 Changed()=true，否则 buildClientOpts 走 env fallback。
	if err := cmd.Flags().Set("base-url", srv.URL); err != nil {
		t.Fatalf("set base-url flag: %v", err)
	}
	cmd.Flags().Int("timeout", 5, "")
	return cmd
}

// captureStdio 替换 os.Stdout/os.Stderr 并返回还原函数 + 同步等待机制。
// 调用模式
//
//	stdoutBuf, stderrBuf, restore := captureStdio(t)
//	defer restore()
//	// 触发命令（命令内对 os.Stdout/Stderr 的写会进入管道）
//	whoamiCmd.Run(cmd, nil)
//	// restore() 同步等待 drain 完成；之后 stdoutBuf/stderrBuf 才包含全部数据
//	stdout := stdoutBuf.String()
//	stderr := stderrBuf.String()
//
// 设计要点
//   - 启动 drain goroutine **前**先 close writer（确保 io.Copy 看到 EOF）
//   - 用 done channel 同步等待 drain goroutine 退出，避免调用方读到空 buffer
//   - 先恢复 os.Stdout/Stderr 再 return，防止后续 t.Logf 等打到管道里
func captureStdio(t *testing.T) (stdout *bytes.Buffer, stderr *bytes.Buffer, restore func()) {
	t.Helper()
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	origStdout, origStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = wOut, wErr

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	return stdoutBuf, stderrBuf, func() {
		// 先关闭 writer 端（确保 io.Copy 能看到 EOF）
		_ = wOut.Close()
		_ = wErr.Close()

		// 并行 drain 两个 reader
		var outDone, errDone bool
		done := make(chan struct{}, 2)
		go func() {
			_, _ = io.Copy(stdoutBuf, rOut)
			done <- struct{}{}
		}()
		go func() {
			_, _ = io.Copy(stderrBuf, rErr)
			done <- struct{}{}
		}()
		for i := 0; i < 2; i++ {
			<-done
		}
		_ = outDone
		_ = errDone

		// 恢复原 stdout/stderr
		os.Stdout = origStdout
		os.Stderr = origStderr
	}
}

// TestWhoami_OkEmpty_StatusEnvelope 回归测试 F5
// GetMyInfo 返回 (nil, nil) 时输出 {"status":"empty","reason":"get_my_info_empty"}。
func TestWhoami_OkEmpty_StatusEnvelope(t *testing.T) {
	cmd := makeWhoamiTestCmd(t, "test-token", "")

	quiet = false
	_ = os.Unsetenv("NAZHI_USERNAME")
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	whoamiCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// exit=0：envelope.Empty 是 success（204），ExitCode=0
	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("空响应 whoami 应触发 pendingExitCode=0（envelope.Empty 成功），实际 %d", got)
	}

	// stderr 不应输出错误 envelope（envelope.Empty 走 stdout）
	if strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("stderr 不应包含 error envelope，实际: %q", stderr)
	}

	// stdout 不应是裸 null
	if strings.TrimSpace(stdout) == "null" {
		t.Errorf("stdout 不应输出裸 null（空响应应输出 envelope.Empty），实际: %q", stdout)
	}

	// stdout 应包含 status=success / code=204（envelope.Empty 表达空数据）
	if !strings.Contains(stdout, `"status": "success"`) {
		t.Errorf("stdout 应包含 status=success（envelope.Empty），实际: %q", stdout)
	}

	// stdout 应包含 code=204
	if !strings.Contains(stdout, `"code": 204`) {
		t.Errorf("stdout 应包含 code=204，实际: %q", stdout)
	}
}

// TestWhoami_Normal_OutputsUserInfo 验证正常 whoami 响应直接输出 UserInfo envelope。
func TestWhoami_Normal_OutputsUserInfo(t *testing.T) {
	cmd := makeWhoamiTestCmd(t, "test-token", unifiedUserInfo)

	quiet = false
	_ = os.Unsetenv("NAZHI_USERNAME")
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	whoamiCmd.Run(cmd, nil)
	restore()
	stdout := stdoutBuf.String()
	_ = stderrBuf.String()

	// 输出 UserInfo（嵌套在 envelope data 字段里）
	if !strings.Contains(stdout, `"name": "张三"`) {
		t.Errorf("正常响应 stdout 应包含 name: 张三，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"schoolName": "福清一中"`) {
		t.Errorf("正常响应 stdout 应包含 schoolName: 福清一中，实际: %q", stdout)
	}
}

// TestWhoami_BizFail_PrintsError 验证业务失败时 whoami 走 printError 路径。
func TestWhoami_BizFail_PrintsError(t *testing.T) {
	// bizOK=false 让 session 预热（首页+/getMenu）都失败
	cmd := makeWhoamiTestCmd(t, "invalid-token", "", false)

	quiet = false
	_ = os.Unsetenv("NAZHI_USERNAME")
	pendingExitCode.Store(0)

	stdoutBuf, stderrBuf, restore := captureStdio(t)
	whoamiCmd.Run(cmd, nil)
	restore()
	_ = stdoutBuf.String()
	stderr := stderrBuf.String()

	// 退出码标记为 2（业务错误 5xx 走 envelope.ExitCode=2）
	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("业务失败 whoami 应标记 pendingExitCode=2（envelope.Error 5xx），实际 %d", got)
	}

	// stderr 含 error envelope
	if !strings.Contains(stderr, `"status": "error"`) {
		t.Errorf("业务失败 whoami stderr 应含 error envelope，实际: %q", stderr)
	}
}

// 静默：防止 import 未使用
var _ = fmt.Sprintf
