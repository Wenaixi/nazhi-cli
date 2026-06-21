package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// selfEvalStatusCmd 表示 nazhi self-eval status 命令
//
//	nazhi self-eval status --token <token> [--base-url <url>] [--timeout <秒>]
var selfEvalStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查询自我评价状态",
	Long:  `查询自我评价提交状态以及教师评语。`,
	Example: `  nazhi self-eval status --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi self-eval status --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		token, _ := cmd.Flags().GetString("token")
		baseURL, _ := cmd.Flags().GetString("base-url")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		if token == "" {
			printError(fmt.Errorf("--token 为必填"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if baseURL != "" {
			opts = append(opts, client.WithBaseURL(baseURL))
		}
		c := client.New(opts...)

		printVerbose("正在查询自我评价状态...")
		status, err := c.QuerySelfEvaluation(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("查询自我评价失败: %w", err))
			return
		}

		printJSON(status)
	},
}

func init() {
	selfEvalStatusCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	selfEvalStatusCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	selfEvalStatusCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
