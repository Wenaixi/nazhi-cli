package main

import (
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
