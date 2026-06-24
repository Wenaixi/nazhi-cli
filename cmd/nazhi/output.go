package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
)

// pendingExitCode 追踪本进程是否遇到错误。
// F7 修复：printError 不再调 os.Exit（否则绕过 main 中 defer closeAllClients()），
// 但 cobra Run 回调 `printError(err); return` 不会让 rootCmd.Execute() 返回
// 非 nil error，于是退出码恒为 0，CI 脚本无法区分成败。
// 这里用 atomic.Int32 让 printError 标记、main 读取，保证：
//   - 退出码语义保持原样（出过错则 exit 1）
//   - defer closeAllClients() 仍能跑（os.Exit 只在 main 最后调一次）
var pendingExitCode atomic.Int32

// markError 标记本进程遇到错误，main 退出时检查。
func markError() {
	pendingExitCode.Store(1)
}

// printJSON 输出 JSON 到 stdout。
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil && !quiet {
		printError(fmt.Errorf("序列化输出失败: %w", err))
	}
}

// printError 输出错误 JSON 到 stderr 并标记退出码为 1。
//
// 注意：此函数**不**调用 os.Exit。退出由 main 在 rootCmd.Execute() 之后
// 统一处理。原因：F7 修复——os.Exit 绕过 goroutine 栈展开，导致 main 的
// defer closeAllClients() 永远不执行，ONNX session + 临时目录 +
// keep-alive 连接全部泄漏。
//
// 退出码契约：
//   - printError 仅写 stderr + 设 pendingExitCode=1，然后 return
//   - 调用方（cobra Run 回调）保持原样 `printError(err); return`
//   - main 在 Execute 返回非 nil 或 pendingExitCode=1 时统一 os.Exit(1)
func printError(err error) {
	markError()
	type errOutput struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if !quiet {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		if enc.Encode(errOutput{Error: true, Message: err.Error()}) != nil {
			// 兜底：JSON 编码失败时直接打印
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		}
	}
}

// printVerbose 输出日志到 stderr（仅在 verbose 模式下且非 quiet）。
func printVerbose(format string, args ...any) {
	if verbose && !quiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
