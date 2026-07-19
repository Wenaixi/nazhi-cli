package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
)

// pendingExitCode 追踪本进程退出码（语义退出码，非 0/1 二元）。
// 从 1.0.0 起，三分退出码：0 成功 / 1 partial/业务 / 2 服务端 / 3 参数。
// printError 不再调 os.Exit（否则绕过 main 中 defer closeAllClients()）。
// 这里用 atomic.Int32 让 printEnvelope/printError 标记、main 读取，保证
//   - 退出码语义保持原样（出过错则非 0）
//   - defer closeAllClients() 仍能跑（os.Exit 只在 main 最后调一次）
var pendingExitCode atomic.Int32

// printErrorDepth 防止 修复中递归调用自身造成死循环。
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

// printError 输出 envelope.Error 到 stderr 并标记退出码。
// 注意：此函数**不**调用 os.Exit。退出由 main 在 rootCmd.Execute() 之后
// 统一处理。原因：修复——os.Exit 绕过 goroutine 栈展开，导致 main 的
// defer closeAllClients() 永远不执行，ONNX session + 临时目录 +
// keep-alive 连接全部泄漏。
// 退出码契约
//   - printError 仅写 stderr + 设 pendingExitCode（按 envelope.ExitCode）
//   - 调用方（cobra Run 回调）保持原样 `printError(err); return`
//   - main 在 Execute 返回非 nil 或 pendingExitCode!=0 时统一 os.Exit
func printError(err error) {
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
	// 默认 code=500（服务端错误兜底），具体命令可调 envelope.Error(code, msg)
	// 自行设置更精确的状态码。
	e := envelope.Error(500, err.Error())

	// quiet 模式下只标记退出码，不写 stderr
	if !quiet {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		if enc.Encode(e) != nil {
			// 兜底：JSON 编码失败时也必须走 pendingExitCode=1 路径
			printError(fmt.Errorf("printError JSON 编码失败: %w", err))
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

// printPrompt 向 stderr 写入交互提示，**不**受 verbose/quiet 守卫。
// 用途：self-eval submit 等从 stdin 读取输入的命令，需要在用户终端看到
// "请输入 xxx: " 提示符才能知道要敲字。如果走 printVerbose（受 verbose 守卫）
// 用户没加 -v 就看不到提示；如果走 printError（受 quiet 守卫 + 走 JSON envelope）
// 又会污染 stderr 错误流。
// 守卫
//   - 仅在 isTerminalStdin()==true 时输出（CI / 管道环境下无意义）
//   - quiet 模式也不输出（用户显式要求静默）
//
// self_eval_submit.go:35 原生
// `fmt.Fprint(os.Stderr, ...)` 绕过统一通道，且不看 quiet，统一收口到本函数。
// 新增「交互提示例外」条款到 CLAUDE.md / env.go 注释。
func printPrompt(prompt string) {
	if quiet {
		return
	}
	if !isTerminalStdin() {
		return
	}
	fmt.Fprint(os.Stderr, prompt)
}
