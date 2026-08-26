package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskSubmittedCmd 表示 nazhi task submitted 命令
//
//	nazhi task submitted --token <token> [--base-url <url>] [--timeout <秒>]
//	nazhi task submitted --limit 20 --offset 10
//	nazhi task submitted --count
//	nazhi task submitted --key 关键词
//
// 同时作为 `task done` 别名注册（语义更直白）。
// type=3：我发布的写实（仅当前用户自己发布的内容）。
var taskSubmittedCmd = &cobra.Command{
	Use:   "submitted",
	Short: "查看我发布的写实记录",
	Long: `调用 getStudentCircle 接口(type=3)，获取当前用户自己发布的全部写实记录。
自动翻页合并，输出全量数据。

支持 --limit / --offset 分批拉取，--count 只看总数，--key 关键字筛选。`,
	Example: `  nazhi task submitted --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task done --token eyJhbGciOiJIUzI1NiJ9.xxx      # 同 submitted，别名
  nazhi task submitted --limit 5                          # 前 5 条
  nazhi task submitted --offset 5 --limit 5               # 第 6~10 条
  nazhi task submitted --count                            # 只看总数
  nazhi task submitted --key 劳动                           # 按关键字筛选`,
	Args: cobra.NoArgs,
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
			printVerbose("正在获取记录总数...")
			total, err := c.PeekSubmittedTotal(cmd.Context(), token, key)
			if err != nil {
				printError(fmt.Errorf("获取记录总数失败: %w", err))
				return
			}
			printEnvelope(envelope.Success(map[string]int{"total": total}))
			return
		}

		if rejectLoneOffset(cmd) {
			return
		}

		if offset > 0 || limit > 0 {
			printVerbose("正在获取我发布的写实记录（limit=%d, offset=%d）...", limit, offset)
			raw, pb, err := c.GetSubmittedCirclesLimitJSON(cmd.Context(), token, offset, limit, key)
			if err != nil {
				if len(raw) > 0 {
					total := 0
					if pb != nil {
						total = pb.TotalNum
					}
					printEnvelope(envelope.Partial(207, "获取我发布的写实记录失败: "+err.Error(), map[string]any{
						"records": json.RawMessage(raw),
						"total":   total,
					}))
					return
				}
				printError(fmt.Errorf("获取我发布的写实记录失败: %w", err))
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

		printVerbose("正在获取我发布的写实记录...")
		raw, err := c.GetSubmittedCirclesJSON(cmd.Context(), token, key)
		if err != nil {
			if len(raw) > 0 {
				printEnvelope(envelope.Partial(207, "获取我发布的写实记录失败: "+err.Error(), json.RawMessage(raw)))
				return
			}
			printError(fmt.Errorf("获取我发布的写实记录失败: %w", err))
			return
		}
		if len(raw) == 0 {
			printEnvelope(envelope.Success(json.RawMessage("[]")))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// taskDoneCmd 是 task submitted 的别名（语义更直白）。
// 共用 taskSubmittedCmd.Run 回调，避免逻辑重复。
var taskDoneCmd = &cobra.Command{
	Use:   "done",
	Short: "查看我发布的写实记录 （task submitted 别名）",
	Args:  cobra.NoArgs,
	Run:   taskSubmittedCmd.Run,
}

func init() {
	registerBizFlags(taskSubmittedCmd)
	registerBizFlags(taskDoneCmd)
	taskSubmittedCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskSubmittedCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskSubmittedCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskSubmittedCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
	// done 别名也需要注册 flag，否则 cobra 解析不认识
	taskDoneCmd.Flags().Int("offset", 0, "跳过前 N 条（配合 --limit 使用）")
	taskDoneCmd.Flags().Int("limit", 0, "只输出前 N 条（0 表示全量）")
	taskDoneCmd.Flags().Bool("count", false, "只输出记录总数，不拉列表")
	taskDoneCmd.Flags().String("key", "", "搜索关键字（可空，对应 getStudentCircle 的 key）")
}
