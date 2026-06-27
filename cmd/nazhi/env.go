package main

import "os"

// isTerminalStdin 检查 stdin 是否连接到真实终端（而非管道或重定向）。
// 用于 stdin 交互提示：CI 环境是管道，直接读取不阻塞。
func isTerminalStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
