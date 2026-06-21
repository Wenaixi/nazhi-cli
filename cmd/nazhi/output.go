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
	if err := enc.Encode(v); err != nil {
		printError(fmt.Errorf("序列化输出失败: %w", err))
	}
}

// printError 输出错误 JSON 到 stderr 并设置 exit code。
func printError(err error) {
	type errOutput struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	enc.Encode(errOutput{Error: true, Message: err.Error()})
	os.Exit(1)
}

// printVerbose 输出日志到 stderr（仅在 verbose 模式下）。
func printVerbose(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
