package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
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
			printVerbose("正在获取公示写实记录总数...")
			total, err := c.PeekPublicTotal(cmd.Context(), token, key)
			if err != nil {
				printError(fmt.Errorf("获取公示写实记录总数失败: %w", err))
				return
			}
			printEnvelope(envelope.Success(map[string]int{"total": total}))
			return
		}

		if offset > 0 || limit > 0 {
			printVerbose("正在获取公示写实记录（limit=%d, offset=%d）...", limit, offset)
			raw, pb, err := c.GetPublicCirclesLimitJSON(cmd.Context(), token, offset, limit, key)
			if err != nil {
				if len(raw) > 0 {
					total := 0
					if pb != nil {
						total = pb.TotalNum
					}
					printEnvelope(envelope.Partial(207, "获取公示写实记录失败: "+err.Error(), map[string]any{
						"records": json.RawMessage(raw),
						"total":   total,
					}))
					return
				}
				printError(fmt.Errorf("获取公示写实记录失败: %w", err))
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

		printVerbose("正在获取公示写实记录...")
		raw, err := c.GetPublicCirclesJSON(cmd.Context(), token, key)
		if err != nil {
			if len(raw) > 0 {
				printEnvelope(envelope.Partial(207, "获取公示写实记录失败: "+err.Error(), json.RawMessage(raw)))
				return
			}
			printError(fmt.Errorf("获取公示写实记录失败: %w", err))
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
	registerBizFlags(taskPublicCmd)
	taskPublicCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskPublicCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskPublicCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskPublicCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
}
