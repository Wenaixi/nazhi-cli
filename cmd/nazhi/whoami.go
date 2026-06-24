package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// whoamiCmd 表示 nazhi whoami 命令
//
//	nazhi whoami --token <token> [--base-url <url>] [--timeout <秒>]
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "获取当前登录用户完整信息",
	Long:  `获取用户的完整个人资料，包括姓名、性别、学号、学校、年级、班级、座号等。`,
	Example: `  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx
	  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取用户信息...")
		info, err := c.GetMyInfo(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取用户信息失败: %w", err))
			return
		}
		// SDK "最佳努力设计"：GetMyInfo 成功但业务未返回用户数据时返回 (nil, nil)，
		// 这不是错误——输出空 JSON（null）让上层管道/脚本正常处理。

		printJSON(info)
	},
}

func init() {
	whoamiCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	whoamiCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	whoamiCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
