package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// schoolCmd 表示 nazhi school 命令
//
//	nazhi school -u <username> [--sso-base <url>] [--timeout <秒>]
var schoolCmd = &cobra.Command{
	Use:   "school",
	Short: "查询学校 ID（不登录）",
	Long:  `根据学号查询对应的学校 ID 和学校名称。不需要登录，只需建立 SSO Session。`,
	Example: `  nazhi school -u 学号
  nazhi school -u 学号 --sso-base https://www.nazhisoft.com`,
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		ssoBase, _ := cmd.Flags().GetString("sso-base")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		// 环境变量 fallback
		if username == "" {
			username = envString("NAZHI_USERNAME", "")
		}
		if ssoBase == "" {
			ssoBase = envString("NAZHI_SSO_BASE", "")
		}
		if !flagChanged(cmd, "timeout") {
			timeoutSec = envInt("NAZHI_TIMEOUT", 15)
		}

		if username == "" {
			printError(fmt.Errorf("--username 为必填（也可通过 NAZHI_USERNAME 环境变量设置）"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if ssoBase != "" {
			opts = append(opts, client.WithSSOBase(ssoBase))
		}
		c, err := client.New(opts...)
		if err != nil {
			printError(fmt.Errorf("构造 Client 失败: %w", err))
			return
		}

		printVerbose("查询学校信息...")
		schoolID, schoolName, err := c.GetSchoolID(cmd.Context(), username)
		if err != nil {
			printError(fmt.Errorf("查询学校 ID 失败: %w", err))
			return
		}

		printJSON(map[string]string{
			"school_id":   schoolID,
			"school_name": schoolName,
		})
	},
}

func init() {
	schoolCmd.Flags().StringP("username", "u", "", "学号（必填）")
	schoolCmd.Flags().String("sso-base", "", "SSO 根地址（默认 https://www.nazhisoft.com）")
	schoolCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
