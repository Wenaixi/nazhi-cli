package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// sessionCmd 表示 nazhi session activate 命令
//
//	nazhi session activate --token <token> [--base-url <url>] [--timeout <秒>]
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "管理业务 Session",
	Long:  `初始化目标平台业务 Session。必须先 GET / + GET /api/studentInfo/getMenu，否则后续接口返回空数据。`,
}

var sessionActivateCmd = &cobra.Command{
	Use:   "activate",
	Short: "激活业务 Session",
	Long:  `使用 token 激活目标平台业务 Session。返回用户基本信息。`,
	Example: `  nazhi session activate --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi session activate --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
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
		if !flagChanged(cmd, "timeout") {
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

		printVerbose("激活 Session...")
		info, err := c.ActivateSession(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("激活 Session 失败: %w", err))
			return
		}

		printJSON(info)
	},
}

func init() {
	sessionCmd.AddCommand(sessionActivateCmd)

	sessionActivateCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	sessionActivateCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	sessionActivateCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
