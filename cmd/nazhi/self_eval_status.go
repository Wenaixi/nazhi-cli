package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// selfEvalStatusCmd 表示 nazhi self-eval status 命令
//
//	nazhi self-eval status --token <token> [--base-url <url>] [--timeout <秒>]
//
// envelope.data 直接透传 SDK QuerySelfEvaluationJSON 的原始 JSON，
// 与平台响应 1:1（保留服务端可能新增的字段）。
var selfEvalStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查询自我评价状态",
	Args:  cobra.NoArgs,
	Long:  `查询自我评价提交状态。输出为原始 JSON 透传（returnData → dataList[0] → dataMap 优先级），字段以平台响应为准；结构化教师评语请用 SDK QuerySelfEvaluation。`,
	Example: `  nazhi self-eval status --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi self-eval status --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在查询自我评价状态...")
		raw, err := c.QuerySelfEvaluationJSON(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("查询自我评价失败: %w", err))
			return
		}
		// 未提交时服务端可能返回空数据，走 Empty(204) envelope 表达。
		if len(raw) == 0 {
			printEnvelope(envelope.Empty("尚未提交自我评价"))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	registerBizFlags(selfEvalStatusCmd)
}
