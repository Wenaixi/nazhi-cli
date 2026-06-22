package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// taskListCmd 表示 nazhi task list 命令
//
//	nazhi task list --token <token> [--base-url <url>] [--timeout <秒>]
var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取全维度任务列表",
	Long:  `拉取目标平台全部维度的任务列表。内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。`,
	Example: `  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
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
		if timeoutSec == 15 {
			timeoutSec = envInt("NAZHI_TIMEOUT", 15)
		}

		if token == "" {
			printError(fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if baseURL != "" {
			opts = append(opts, client.WithBaseURL(baseURL))
		}
		if token != "" {
			opts = append(opts, client.WithToken(token))
		}
		c := client.New(opts...)

		printVerbose("正在获取任务列表...")
		tasks, err := c.FetchTasks(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取任务列表失败: %w", err))
			return
		}

		printJSON(tasks)
	},
}

func init() {
	taskListCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	taskListCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	taskListCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
