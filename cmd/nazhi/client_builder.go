package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// buildBizClient 从 cobra 命令标志构建 Client，自动处理 env fallback。
//
// 必填标志：token。
// 可选标志：base-url, timeout。
//
// 返回 (client, token)。
func buildBizClient(cmd *cobra.Command) (*client.Client, string, error) {
	token, _ := cmd.Flags().GetString("token")
	baseURL, _ := cmd.Flags().GetString("base-url")
	timeoutSec, _ := cmd.Flags().GetInt("timeout")

	// 环境变量 fallback
	if token == "" {
		token = envString("NAZHI_TOKEN", "")
	}
	if baseURL == "" {
		baseURL = envString("NAZHI_BASE_URL", "")
	}
	if !flagChanged(cmd, "timeout") {
		timeoutSec = envInt("NAZHI_TIMEOUT", 15)
	}

	if token == "" {
		return nil, "", fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）")
	}

	opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
	if baseURL != "" {
		opts = append(opts, client.WithBaseURL(baseURL))
	}
	if token != "" {
		opts = append(opts, client.WithToken(token))
	}
	return client.New(opts...), token, nil
}
