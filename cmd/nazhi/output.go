package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// printJSON 输出 JSON 到 stdout。
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil && !quiet {
		printError(fmt.Errorf("序列化输出失败: %w", err))
	}
}

// printError 输出错误 JSON 到 stderr 并设置 exit code。
// 即使 stderr 写入失败，也确保退到 fmt.Fprintln 兜底。
func printError(err error) {
	type errOutput struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if !quiet {
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		if enc.Encode(errOutput{Error: true, Message: err.Error()}) == nil {
			os.Exit(1)
			return
		}
		// 兜底：JSON 编码失败时直接打印
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
	}
	os.Exit(1)
}

// printVerbose 输出日志到 stderr（仅在 verbose 模式下且非 quiet）。
func printVerbose(format string, args ...any) {
	if verbose && !quiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
