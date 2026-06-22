package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
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

// envDuration 返回解析后的 time.Duration 环境变量值（按秒），失败或未设置则返回 fallback。
func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}

// requireEnv 检查必备环境变量，全部存在则返回 nil，否则返回包含缺失项的 error。
func requireEnv(keys ...string) error {
	var missing []string
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); !ok || v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("缺少环境变量: %v（可通过 --flag 或 .env 注入）", missing)
	}
	return nil
}
