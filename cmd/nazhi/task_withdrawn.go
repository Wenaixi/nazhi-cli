package main

import (
	"github.com/spf13/cobra"
)

// taskWithdrawnCmd 表示 nazhi task withdrawn 命令
//
//	nazhi task withdrawn --token <token> [--base-url <url>] [--timeout <秒>]
//	nazhi task withdrawn --limit 20 --offset 10
//	nazhi task withdrawn --count
//	nazhi task withdrawn --key 关键词
//
// type=4：被撤回的写实记录。
var taskWithdrawnCmd = &cobra.Command{
	Use:   "withdrawn",
	Short: "获取被撤回的写实记录",
	Long: `调用 getStudentCircle 接口(type=4)，获取被审核撤回的全部写实记录。
自动翻页合并，输出全量数据。

支持 --limit / --offset 分批拉取，--count 只看总数，--key 关键字筛选。`,
	Example: `  nazhi task withdrawn --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task withdrawn --limit 5                          # 前 5 条
  nazhi task withdrawn --offset 5 --limit 5               # 第 6~10 条
  nazhi task withdrawn --count                            # 只看总数
  nazhi task withdrawn --key 劳动                           # 按关键字筛选`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		withdrawnCircleListMode.run(cmd)
	},
}

func init() {
	registerBizFlags(taskWithdrawnCmd)
	taskWithdrawnCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskWithdrawnCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskWithdrawnCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskWithdrawnCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
}
