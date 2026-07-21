package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
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
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		onlyCount, _ := cmd.Flags().GetBool("count")
		offset, _ := cmd.Flags().GetInt("offset")
		limit, _ := cmd.Flags().GetInt("limit")
		key, _ := cmd.Flags().GetString("key")

		if onlyCount {
			printVerbose("正在获取被撤回写实记录总数...")
			total, err := c.PeekWithdrawnTotal(cmd.Context(), token, key)
			if err != nil {
				printError(fmt.Errorf("获取被撤回写实记录总数失败: %w", err))
				return
			}
			printEnvelope(envelope.Success(map[string]int{"total": total}))
			return
		}

		if offset > 0 || limit > 0 {
			printVerbose("正在获取被撤回写实记录（limit=%d, offset=%d）...", limit, offset)
			raw, pb, err := c.GetWithdrawnCirclesLimitJSON(cmd.Context(), token, offset, limit, key)
			if err != nil {
				if len(raw) > 0 {
					total := 0
					if pb != nil {
						total = pb.TotalNum
					}
					printEnvelope(envelope.Partial(207, "获取被撤回写实记录失败: "+err.Error(), map[string]any{
						"records": json.RawMessage(raw),
						"total":   total,
					}))
					return
				}
				printError(fmt.Errorf("获取被撤回写实记录失败: %w", err))
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

		printVerbose("正在获取被撤回写实记录...")
		raw, err := c.GetWithdrawnCirclesJSON(cmd.Context(), token, key)
		if err != nil {
			if len(raw) > 0 {
				printEnvelope(envelope.Partial(207, "获取被撤回写实记录失败: "+err.Error(), json.RawMessage(raw)))
				return
			}
			printError(fmt.Errorf("获取被撤回写实记录失败: %w", err))
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
	registerBizFlags(taskWithdrawnCmd)
	taskWithdrawnCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskWithdrawnCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskWithdrawnCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskWithdrawnCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
}
