package main

import (
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// isTerminalStdin 检查 stdin 是否连接到真实终端（而非管道或重定向）。
// 用于 stdin 交互提示：CI 环境是管道，直接读取不阻塞。
func isTerminalStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// envString 返回环境变量值，若未设置或为空则返回 fallback。
func envString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// envInt 返回解析后的 int 环境变量值，失败或未设置则返回 fallback。
func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// flagChanged 检查用户是否通过命令行显式设置了某个 flag。
// 用于避免"哨兵默认值"反模式：用户传 --timeout 15 时不应被环境变量覆盖。
func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	return cmd.Flags().Changed(name)
}

// applyURLFlag 按设计契约读取 flag 值
//   - flagChanged(cmd, flagName)==true → 用户显式传了 flag，用 flag 值（含显式空字符串）
//   - flagChanged(cmd, flagName)==false → 未传 flag，走 env fallback
//
// 消除 buildClientOpts 中 6 处重复的
// 「flagChanged + GetString + envString」模板，统一收口到本函数。
func applyURLFlag(cmd *cobra.Command, flagName, envKey string) string {
	if flagChanged(cmd, flagName) {
		v, _ := cmd.Flags().GetString(flagName)
		return v
	}
	return envString(envKey, "")
}
