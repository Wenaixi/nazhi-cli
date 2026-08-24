package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/spf13/cobra"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
)

// pendingExitCode 追踪本进程退出码。三分退出码：0 成功 / 1 partial/业务 / 2 服务端 / 3 参数。
// printError 不再调 os.Exit（否则绕过 main 中 defer closeAllClients()）。
// 这里用 atomic.Int32 让 printEnvelope/printError 标记、main 读取，保证
//   - 退出码语义保持原样（出过错则非 0）
//   - defer closeAllClients() 仍能跑（os.Exit 只在 main 最后调一次）
var pendingExitCode atomic.Int32

// printErrorDepth 防止递归兜底路径无限递归。
// 当 stderr 本身也无法 JSON 编码时（如 fd 已关），递归兜底会无限递归。
// depth>1 时降级为直写 fmt.Fprintf，避免 stack overflow。
var printErrorDepth atomic.Int32

// markError 标记本进程遇到错误（默认退出码 1）。
// 若需要更精细的退出码（业务错误 1 / 服务端 2 / 参数 3），应直接调
// pendingExitCode.Store(...) 或 printEnvelope 让 envelope.ExitCode() 接管。
func markError() {
	pendingExitCode.Store(1)
}

// printEnvelope 序列化 envelope 到 stdout 并按 ExitCode 标记退出码。
// 这是 CLI 所有 Run 回调的统一出口。
func printEnvelope(e *envelope.Envelope) {
	if e == nil {
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(e); err != nil {
		markError()
		if !quiet {
			printError(fmt.Errorf("序列化 envelope 失败: %w", err))
		}
		return
	}
	if code := e.ExitCode(); code != 0 {
		pendingExitCode.Store(int32(code))
	}
}

// mapSentinelToHTTPCode 按错误链中的哨兵归类 HTTP 码：
// 参数类（含本地文件超限 ErrFileTooLarge）→ 400（exit 3）；业务拒绝/服务端明确 4xx → 422（exit 1）；
// 限流 → 429（exit 1）；网络/超时/5xx → 502（exit 2）；未识别保持 500。
// 修复业务拒绝被压成退出码 2、与瞬时故障不可区分的问题。
func mapSentinelToHTTPCode(err error) int {
	switch {
	case errors.Is(err, client.ErrInvalidPayload),
		errors.Is(err, client.ErrFileTooLarge):
		return 400
	case errors.Is(err, client.ErrBusinessRejected),
		errors.Is(err, client.ErrLoginRejected),
		errors.Is(err, client.ErrInvalidResponse),
		errors.Is(err, client.ErrUploadRejected):
		return 422
	case errors.Is(err, client.ErrRateLimited):
		return 429
	case errors.Is(err, client.ErrNetwork),
		errors.Is(err, client.ErrTimeout),
		errors.Is(err, client.ErrServiceUnavailable):
		return 502
	default:
		return 500
	}
}

// printError 输出错误信封到 stderr：按哨兵映射 HTTP 码决定退出码档位。
// 注意：此函数**不**调用 os.Exit。退出由 main 在 rootCmd.Execute() 之后
// 统一处理。原因：os.Exit 不执行 defer，直接退出会导致 main 的
// defer closeAllClients() 永远不运行，HTTP 连接池等资源全部泄漏。
// 退出码契约
//   - printError 仅写 stderr + 设 pendingExitCode（按 envelope.ExitCode）
//   - 调用方（cobra Run 回调）保持原样 `printError(err); return`
//   - main 在 Execute 返回非 nil 或 pendingExitCode!=0 时统一 os.Exit
//
// 参数错误（缺 token、payload 读/解析失败）请用 printParamError（400→exit 3）。
func printError(err error) {
	printErrorWithCode(err, mapSentinelToHTTPCode(err))
}

// printParamError 输出参数错误 envelope.Error(400) 到 stderr，退出码 3。
// 用于 buildBizClient 失败（缺 token）、payload 读取/JSON 解析失败等
// 调用方可控的输入问题，与服务端/网络错误（printError → 500/exit 2）区分。
func printParamError(err error) {
	printErrorWithCode(err, 400)
}

// rejectLoneOffset 校验 --offset 合法性：单独 --offset（无 --limit）或负值时
// 输出参数错误信封并返回 true。offset>0 而 limit<=0 会被 SDK 全量路径静默忽略；
// offset<0 在 limit 模式下等效归零、全量模式下整体失效——分页脚本 page 计算
// 出错时会无声拿到错误切片。四命令统一拒绝以防静默错误数据。
func rejectLoneOffset(cmd *cobra.Command) bool {
	offset, _ := cmd.Flags().GetInt("offset")
	limit, _ := cmd.Flags().GetInt("limit")
	if (offset > 0 && limit <= 0) || offset < 0 {
		printEnvelope(envelope.Error(400, "--offset 需为非负数且配合 --limit 使用（非法 --offset 会被忽略或归零，拒绝静默返回错误数据）"))
		return true
	}
	return false
}

// printErrorWithCode 是 printError / printParamError 的共享实现。
func printErrorWithCode(err error, httpCode int) {
	if err == nil {
		return
	}
	// depth 守卫：递归调用只在 depth==0 时触发，避免 stderr fd 关闭时死循环。
	if printErrorDepth.Add(1) > 1 {
		// 二次调用（兜底路径又失败）→ 直接降级为 fmt.Fprintf，不再递归
		if !quiet {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		}
		printErrorDepth.Add(-1)
		return
	}
	defer printErrorDepth.Add(-1)

	// 把 error 包成 envelope，按 ExitCode 设退出码。
	e := envelope.Error(httpCode, err.Error())

	// quiet 模式下只标记退出码，不写 stderr
	if !quiet {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		if enc.Encode(e) != nil {
			// 兜底：JSON 编码失败时也必须走 pendingExitCode 路径
			printErrorWithCode(fmt.Errorf("printError JSON 编码失败: %w", err), httpCode)
			return
		}
	}
	if code := e.ExitCode(); code != 0 {
		pendingExitCode.Store(int32(code))
	}
}

// printVerbose 输出日志到 stderr（仅在 verbose 模式下且非 quiet）。
// 加 [verbose] 前缀，与 printError JSON envelope 区分
// 避免 verbose 日志被错误接收方误解析为 JSON 错误。
func printVerbose(format string, args ...any) {
	if verbose && !quiet {
		fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
	}
}

// printPrompt 向 stderr 写入交互提示。不受 verbose 守卫，但受 quiet 与终端检测双重守卫。
// 用途：self-eval submit 等从 stdin 读取输入的命令，需要在用户终端看到
// "请输入 xxx: " 提示符才能知道要敲字。走 printVerbose 用户没加 -v 看不到提示；
// 走 printError 会以 JSON envelope 污染 stderr 错误流。
// 守卫：
//   - 仅在 isTerminalStdin()==true 时输出（CI / 管道环境下无意义）
//   - quiet 模式不输出（用户显式要求静默）
func printPrompt(prompt string) {
	if quiet {
		return
	}
	if !isTerminalStdin() {
		return
	}
	fmt.Fprint(os.Stderr, prompt)
}
