package main

import (
	"fmt"

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
		// B1 修复：用 flagChanged() 守卫 username 读取
		// 避免用户显式传 --username "" 时被 NAZHI_USERNAME 环境变量覆盖。
		// 与 login.go:31-35 模式对称。
		var username string
		if flagChanged(cmd, "username") {
			username, _ = cmd.Flags().GetString("username")
		} else {
			username = envString("NAZHI_USERNAME", "")
		}
		if username == "" {
			printError(fmt.Errorf("--username 为必填（也可通过 NAZHI_USERNAME 环境变量设置）"))
			return
		}

		// SSO 命令（login/school）不要求 token，复用 buildClient 共享 env fallback。
		// 消除 inline client.New + 自动获得 trackClient。
		c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
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
