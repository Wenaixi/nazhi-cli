package main

import (
	"github.com/spf13/cobra"
)

// taskSubmittedCmd 表示 nazhi task submitted 命令
//
//	nazhi task submitted --token <token> [--base-url <url>] [--timeout <秒>]
var taskSubmittedCmd = &cobra.Command{
	Use:   "submitted",
	Short: "获取已提交写实记录",
	Long: `调用 getStudentCircle 接口，获取当前用户已提交的全部写实记录（含正文、图片、审核状态）。
自动翻页合并，输出全量数据。`,
	Example: `  nazhi task submitted --token eyJhbGciOiJIUzI1NiJ9.xxx`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取已提交写实记录...")
		records, err := c.GetSubmittedCircles(cmd.Context(), token)
		if err != nil {
			printError(err)
			return
		}

		printJSON(map[string]any{
			"total":   len(records),
			"records": records,
		})
	},
}

func init() {
	registerBizFlags(taskSubmittedCmd)
}
