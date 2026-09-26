package main

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// ─── 全维度 5xx 汇总错误不得误报业务拒绝 422/exit1 ───

// buildFetchTasksAllFailErr 按 FetchTasks「全部维度失败 → 汇总」的真实错误形状构造：
//
//	joined := errors.Join(维度级错误...)   // 每个维度级错误已带网络类哨兵（%w 链）
//	return fmt.Errorf("%w: FetchTasks 全部 %d 个维度均失败: %w", ErrBusinessRejected, n, joined)
func buildFetchTasksAllFailErr(sentinel error) error {
	dimErrs := []error{
		fmt.Errorf("维度 1(思想品德) 请求失败: %w", sentinel),
		fmt.Errorf("维度 2(劳动素养) 请求失败: %w", sentinel),
	}
	joined := errors.Join(dimErrs...)
	return fmt.Errorf("%w: FetchTasks 全部 2 个维度均失败: %w", client.ErrBusinessRejected, joined)
}

// TestMapSentinelToHTTPCode_AllDimsServiceUnavailable_502 服务端整体宕机（全维度 503）时，
// CLI 词法必须先命中 ErrServiceUnavailable 归 502（exit 2），不得先命中 ErrBusinessRejected 归 422（exit 1）。
// 契约：网络/5xx 用稳定哨兵映射 502/503；业务拒绝才是 422。脚本按 422 不重试、按 5xx 退避重放。
func TestMapSentinelToHTTPCode_AllDimsServiceUnavailable_502(t *testing.T) {
	err := buildFetchTasksAllFailErr(client.ErrServiceUnavailable)
	if got := mapSentinelToHTTPCode(err); got != 502 {
		t.Errorf("全维度 503 汇总错误应映射 502，实际 %d（当前误报业务拒绝 422）", got)
	}
}

// TestMapSentinelToHTTPCode_AllDimsRateLimited_429 全维度 429 汇总错误应归 429 而非 422。
func TestMapSentinelToHTTPCode_AllDimsRateLimited_429(t *testing.T) {
	err := buildFetchTasksAllFailErr(client.ErrRateLimited)
	if got := mapSentinelToHTTPCode(err); got != 429 {
		t.Errorf("全维度 429 汇总错误应映射 429，实际 %d", got)
	}
}

// TestMapSentinelToHTTPCode_AllDimsNetwork_502 全维度网络错误汇总应归 502。
func TestMapSentinelToHTTPCode_AllDimsNetwork_502(t *testing.T) {
	err := buildFetchTasksAllFailErr(client.ErrNetwork)
	if got := mapSentinelToHTTPCode(err); got != 502 {
		t.Errorf("全维度网络错误汇总应映射 502，实际 %d", got)
	}
}

// TestMapSentinelToHTTPCode_AllDimsTimeout_502 全维度超时汇总应归 502。
func TestMapSentinelToHTTPCode_AllDimsTimeout_502(t *testing.T) {
	err := buildFetchTasksAllFailErr(client.ErrTimeout)
	if got := mapSentinelToHTTPCode(err); got != 502 {
		t.Errorf("全维度超时汇总应映射 502，实际 %d", got)
	}
}

// TestPrintError_AllDimsServiceUnavailable_Exit2 端到端：printError 收到全维度 503 汇总错误时
// 退出码必须是 2（envelope 502 档），不是 1（422 档）。这是脚本按 exit2 退避重放的关键。
func TestPrintError_AllDimsServiceUnavailable_Exit2(t *testing.T) {
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

	pendingExitCode.Store(0)
	printError(buildFetchTasksAllFailErr(client.ErrServiceUnavailable))
	_ = w.Close()

	if got := pendingExitCode.Load(); got != 2 {
		t.Errorf("全维度 503 汇总错误应 exit 2（502 档），实际 %d（当前误报 exit 1）", got)
	}
}
