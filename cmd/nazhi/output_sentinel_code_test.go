package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestPrintError_BusinessRejected_ExitCode1 业务拒绝（服务端 code!=1）属确定性
// 失败，应归入 4xx 档（退出码 1），与服务端/网络故障（退出码 2）区分；
// 否则依赖退出码重试的脚本会把业务拒绝当瞬时故障盲目重放。
func TestPrintError_BusinessRejected_ExitCode1(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	wrapped := fmt.Errorf("提交任务失败: %w", client.ErrBusinessRejected)
	printError(wrapped)

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("ErrBusinessRejected 应走 4xx 档(退出码 1), 实际 %d", got)
	}
}

// TestPrintError_InvalidPayload_ExitCode3 参数类哨兵 ErrInvalidPayload 属调用方输入问题，
// 应走 400（退出码 3），此前错走 printError(500) → exit 2。
func TestPrintError_InvalidPayload_ExitCode3(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	printError(fmt.Errorf("构造 payload: %w", client.ErrInvalidPayload))

	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("ErrInvalidPayload 应走 400(退出码 3), 实际 %d", got)
	}
}

// TestPrintError_FileTooLarge_ExitCode3 本地文件超限属调用方可控输入问题，
// 应走 400（退出码 3），与服务端/网络故障（退出码 2）区分；
// 否则脚本会把「换个小文件即可」的确定性失败当瞬时故障盲目重试。
func TestPrintError_FileTooLarge_ExitCode3(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	printError(fmt.Errorf("上传失败: %w", client.ErrFileTooLarge))

	if got := pendingExitCode.Load(); got != 3 {
		t.Errorf("ErrFileTooLarge 应走 400(退出码 3), 实际 %d", got)
	}
}

// TestPrintError_Network_StillExit2 网络类哨兵保持退出码 2 不变。
func TestPrintError_Network_StillExit2(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	printError(fmt.Errorf("拉取失败: %w", client.ErrNetwork))

	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("ErrNetwork 应保持退出码 2, 实际 %d", got)
	}
}

// TestPrintError_SessionBackoff_ExitCode1 会话冷却窗口属「客户端已知应等待」的确定性状态，
// 与限流同档归 429（退出码 1）；此前落 default 500/exit2，脚本会把冷却当服务端故障退避重放。
func TestPrintError_SessionBackoff_ExitCode1(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	printError(fmt.Errorf("激活失败: %w", client.ErrSessionBackoff))

	if got := pendingExitCode.Load(); got != 1 {
		t.Errorf("ErrSessionBackoff 应走 429 档(退出码 1), 实际 %d", got)
	}
}

// TestPrintError_Retryable_ExitCode2 ErrRetryable（上下文取消/截止导致的可重试失败）
// 归 503 服务端档（退出码 2），此前落 default 500——语义同档但映射显式化，防误判为未知错误。
func TestPrintError_Retryable_ExitCode2(t *testing.T) {
	orig := pendingExitCode.Load()
	defer pendingExitCode.Store(orig)
	quiet = false

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe 失败: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = origStderr; _ = r.Close() }()

	printError(fmt.Errorf("聚合失败: %w", client.ErrRetryable))

	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("ErrRetryable 应走 503 档(退出码 2), 实际 %d", got)
	}
}
// TestMapSentinelToHTTPCode_ContextCancelledIsRetryable 锁定 context 哨兵的映射。
//
// 用户按 Ctrl+C 中止一条长命令是正常交互，不是服务端内部故障。此前
// context.Canceled/DeadlineExceeded 不在漏斗内，落 default 500；SDK 侧
// ErrRetryable 的注释也明确说它就是「ctx cancel 引发的可重试语义标记」，
// 已映射 503。裸 context 哨兵应与之同档。
func TestMapSentinelToHTTPCode_ContextCancelledIsRetryable(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		if got := mapSentinelToHTTPCode(err); got != 503 {
			t.Errorf("%v 应映射为 503(可重试), 实际 %d", err, got)
		}
	}
}

// TestMapSentinelToHTTPCode_EmptyUserInfoIsServiceSide 锁定空用户信息的映射。
//
// ErrEmptyUserInfo 表示服务端成功响应但没有用户数据（errors.go:57），
// 属服务端侧异常，此前落 default 500。
func TestMapSentinelToHTTPCode_EmptyUserInfoIsServiceSide(t *testing.T) {
	if got := mapSentinelToHTTPCode(client.ErrEmptyUserInfo); got != 502 {
		t.Errorf("ErrEmptyUserInfo 应映射为 502(服务端侧), 实际 %d", got)
	}
}

// TestMapSentinelToHTTPCode_EmptyDecodersFailedIsServiceSide 锁定空成功链路的映射。
//
// ErrAllDecodersFailed 表示业务成功但所有解码器都未命中（errors.go:123），
// 与空用户信息同类：服务端返回了无数据响应。
func TestMapSentinelToHTTPCode_EmptyDecodersFailedIsServiceSide(t *testing.T) {
	if got := mapSentinelToHTTPCode(client.ErrAllDecodersFailed); got != 502 {
		t.Errorf("ErrAllDecodersFailed 应映射为 502(服务端侧), 实际 %d", got)
	}
}
