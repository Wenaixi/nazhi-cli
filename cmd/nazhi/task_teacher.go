package main

import (
	"github.com/spf13/cobra"
)

// taskTeacherCmd 表示 nazhi task teacher 命令
//
//	nazhi task teacher --token <token> [--base-url <url>] [--timeout <秒>]
//	nazhi task teacher --limit 20 --offset 10
//	nazhi task teacher --count
//	nazhi task teacher --key 关键词
//
// type=2：教师写实记录。
var taskTeacherCmd = &cobra.Command{
	Use:   "teacher",
	Short: "获取教师代写的写实记录",
	Long: `调用 getStudentCircle 接口(type=2)，获取教师代写的全部写实记录。
自动翻页合并，输出全量数据。

支持 --limit / --offset 分批拉取，--count 只看总数，--key 关键字筛选。`,
	Example: `  nazhi task teacher --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task teacher --limit 5                          # 前 5 条
  nazhi task teacher --offset 5 --limit 5               # 第 6~10 条
  nazhi task teacher --count                            # 只看总数
  nazhi task teacher --key 劳动                           # 按关键字筛选`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		teacherCircleListMode.run(cmd)
	},
}

func init() {
	registerBizFlags(taskTeacherCmd)
	taskTeacherCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskTeacherCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskTeacherCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskTeacherCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
}
