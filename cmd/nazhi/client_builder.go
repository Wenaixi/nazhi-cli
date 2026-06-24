package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// pendingClients 跟踪本次进程内构造的所有 Client，main 退出前统一 Close()。
// 解决 "Client 包装了 *ocr.Pool 但不暴露 Close() → 临时目录泄漏" 的问题。
var (
	pendingClientsMu sync.Mutex
	pendingClients   []*client.Client
)

// trackClient 把 Client 加入待清理列表。
// 由 buildBizClient / loginCmd 等构造入口调用，业务侧无需感知。
func trackClient(c *client.Client) {
	pendingClientsMu.Lock()
	pendingClients = append(pendingClients, c)
	pendingClientsMu.Unlock()
}

// closeAllClients 关闭所有待清理 Client，返回聚合错误。
// 在 main 函数退出前调用一次 (defer)，保证 ONNX session + 临时目录 + keep-alive 连接全部释放。
func closeAllClients() error {
	pendingClientsMu.Lock()
	clients := pendingClients
	pendingClients = nil
	pendingClientsMu.Unlock()

	var firstErr error
	for _, c := range clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

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
	c := client.New(opts...)
	trackClient(c)
	return c, token, nil
}
