package main

import (
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

// ─── 环境变量约定 ───────────────────────────────────────────
//
// 所有 nazhi CLI 命令支持以下环境变量作为标志的默认值
// （命令行标志始终优先于环境变量）：
//
//   NAZHI_USERNAME     — 学号（login、school）
//   NAZHI_PASSWORD     — 密码（login）
//   NAZHI_TOKEN        — X-Auth-Token（session/whoami/task/self-eval）
//   NAZHI_SSO_BASE     — SSO 根地址（login、school）
//   NAZHI_BASE_URL     — 业务 API 根地址（session/whoami/task/self-eval）
//   NAZHI_TIMEOUT      — HTTP 超时（秒，所有命令）
//   NAZHI_UPLOAD_URL   — 文件上传 API 根地址（file upload）
//
// 推荐在 CI/集成测试中通过 `.env` 文件或 secret 注入。

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

// isTerminalStdin 检查 stdin 是否连接到真实终端（而非管道或重定向）。
// 用于 stdin 交互提示：CI 环境是管道，直接读取不阻塞。
func isTerminalStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
