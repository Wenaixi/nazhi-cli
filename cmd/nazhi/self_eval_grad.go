package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// selfEvalGradStatusCmd 表示 nazhi self-eval grad-status 命令。
//
// envelope.data 直接透传 SDK QuerySelfGradEvaluationJSON 的原始 JSON，
// 与平台响应 1:1（前端 mainLeft.vue 主读 dataMap.student_comment / isGrad）。
var selfEvalGradStatusCmd = &cobra.Command{
	Use:     "grad-status",
	Short:   "查询毕业评价状态",
	Long:    "查询毕业评价内容以及是否显示毕业评价入口（isGrad）。",
	Example: "  nazhi self-eval grad-status --token eyJhbGciOiJIUzI1NiJ9.xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在查询毕业评价状态...")
		raw, err := c.QuerySelfGradEvaluationJSON(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("查询毕业评价失败: %w", err))
			return
		}
		if len(raw) == 0 {
			printEnvelope(envelope.Empty("尚未提交毕业评价"))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// selfEvalGradSubmitCmd 表示 nazhi self-eval grad-submit 命令。
//
// 对齐前端 mainLeft.vue：POST {studentComment: textarea2}。
var selfEvalGradSubmitCmd = &cobra.Command{
	Use:     "grad-submit",
	Short:   "提交毕业评价",
	Long:    "提交毕业评价文本。--comment 为空或为 - 时从 stdin 读取。",
	Example: "  nazhi self-eval grad-submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment \"毕业感言\"",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		comment, _ := cmd.Flags().GetString("comment")

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		if comment == "" || comment == "-" {
			if isTerminalStdin() {
				printPrompt("请输入毕业评价内容（Ctrl+D 结束）: ")
			}
			var readErr error
			comment, readErr = readStdinWithTimeout(cmd.Context(), 60)
			if readErr != nil {
				printError(fmt.Errorf("读取 stdin 评价内容失败: %w", readErr))
				return
			}
			if comment == "" {
				printEnvelope(envelope.Error(400, "评价内容不能为空"))
				return
			}
		}

		printVerbose("正在提交毕业评价...")
		if err := c.SubmitSelfGradEvaluation(cmd.Context(), token, comment); err != nil {
			printError(fmt.Errorf("提交毕业评价失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("毕业评价提交成功"))
	},
}

func init() {
	registerBizFlags(selfEvalGradStatusCmd)
	registerBizFlags(selfEvalGradSubmitCmd)
	selfEvalGradSubmitCmd.Flags().String("comment", "", "毕业评价文本（空或 - 则从 stdin 读取）")
}
