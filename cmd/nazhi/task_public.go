package main

import (
	"github.com/spf13/cobra"
)

// taskPublicCmd 表示 nazhi task public 命令
//
//	nazhi task public --token <token> [--base-url <url>] [--timeout <秒>]
//	nazhi task public --limit 20 --offset 10
//	nazhi task public --count
//	nazhi task public --key 关键词
//
// type=1：公示/全部（全班所有记录）。
var taskPublicCmd = &cobra.Command{
	Use:   "public",
	Short: "获取公示的全部写实记录（全班所有记录）",
	Long: `调用 getStudentCircle 接口(type=1)，获取全班公示/全部写实记录。
自动翻页合并，输出全量数据。

支持 --limit / --offset 分批拉取，--count 只看总数，--key 关键字筛选。`,
	Example: `  nazhi task public --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task public --limit 5                          # 前 5 条
  nazhi task public --offset 5 --limit 5               # 第 6~10 条
  nazhi task public --count                            # 只看总数
  nazhi task public --key 劳动                           # 按关键字筛选`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		publicCircleListMode.run(cmd)
	},
}

func init() {
	registerBizFlags(taskPublicCmd)
	taskPublicCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskPublicCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskPublicCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskPublicCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
}
