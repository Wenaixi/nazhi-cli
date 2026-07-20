package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskTeacherCmd 表示 nazhi task teacher 命令
//
//	nazhi task teacher --token <token> [--base-url <url>] [--timeout <秒>]
//	nazhi task teacher --limit 20 --offset 10
//	nazhi task teacher --count
//
// type=2：教师写实记录。
var taskTeacherCmd = &cobra.Command{
	Use:   "teacher",
	Short: "获取教师代写的写实记录",
	Long: `调用 getStudentCircle 接口(type=2)，获取教师代写的全部写实记录。
自动翻页合并，输出全量数据。

支持 --limit / --offset 分批拉取，--count 只看总数。`,
	Example: `  nazhi task teacher --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task teacher --limit 5                          # 前 5 条
  nazhi task teacher --offset 5 --limit 5               # 第 6~10 条
  nazhi task teacher --count                            # 只看总数`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		onlyCount, _ := cmd.Flags().GetBool("count")
		offset, _ := cmd.Flags().GetInt("offset")
		limit, _ := cmd.Flags().GetInt("limit")

		if onlyCount {
			printVerbose("正在获取教师写实记录总数...")
			total, err := c.PeekTeacherTotal(cmd.Context(), token, "")
			if err != nil {
				printError(fmt.Errorf("获取教师写实记录总数失败: %w", err))
				return
			}
			printEnvelope(envelope.Success(map[string]int{"total": total}))
			return
		}

		if offset > 0 || limit > 0 {
			printVerbose("正在获取教师写实记录（limit=%d, offset=%d）...", limit, offset)
			raw, pb, err := c.GetTeacherCirclesLimitJSON(cmd.Context(), token, offset, limit)
			if err != nil {
				printError(fmt.Errorf("获取教师写实记录失败: %w", err))
				return
			}
			total := 0
			if pb != nil {
				total = pb.TotalNum
			}
			printEnvelope(envelope.Success(map[string]any{
				"records": json.RawMessage(raw),
				"total":   total,
			}))
			return
		}

		printVerbose("正在获取教师写实记录...")
		raw, err := c.GetTeacherCirclesJSON(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取教师写实记录失败: %w", err))
			return
		}
		if len(raw) == 0 {
			printEnvelope(envelope.Success(json.RawMessage("[]")))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	registerBizFlags(taskTeacherCmd)
	taskTeacherCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskTeacherCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskTeacherCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
}
