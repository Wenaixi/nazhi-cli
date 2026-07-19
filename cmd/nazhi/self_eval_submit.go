package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// selfEvalSubmitCmd 表示 nazhi self-eval submit 命令
//
//	nazhi self-eval submit --token <token> --comment "<评价>" [--base-url <url>] [--timeout <秒>]
//	nazhi self-eval submit --token <token> --payload '<json>'    # 结构化提交
var selfEvalSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交自我评价",
	Long: `提交自我评价文本。支持两种模式：

  1. 纯文本模式：--comment 参数（默认）。如果 --comment 为空或为 "-"，则从 stdin 读取。
  2. 结构化模式：--payload 参数，接收 JSON 对象（对应前端"诉得失"页面的完整结构化评价）。
     SDK 内部自动做双层 JSON 包装后提交。`,
	Example: `  nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "很好的学期"
  nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "-"
  nazhi self-eval submit --token xxx --payload '{"bxqhzr":"会做人目标","bxqbx":"表现","bxqys":"优势"}'`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		payloadRaw, _ := cmd.Flags().GetString("payload")
		comment, _ := cmd.Flags().GetString("comment")

		// 结构化模式：--payload 优先
		if payloadRaw != "" {
			payloadBytes, err := parsePayloadFromArg(payloadRaw)
			if err != nil {
				printError(fmt.Errorf("读取 payload 失败: %w", err))
				return
			}
			var form map[string]any
			if err := json.Unmarshal(payloadBytes, &form); err != nil {
				printError(fmt.Errorf("解析 payload JSON 失败: %w", err))
				return
			}

			printVerbose("正在提交结构化自我评价...")
			if err := c.SubmitSelfEvaluationStructured(cmd.Context(), token, form); err != nil {
				printError(fmt.Errorf("提交结构化自我评价失败: %w", err))
				return
			}
			printEnvelope(envelope.Empty("结构化自我评价提交成功"))
			return
		}

		// 纯文本模式：--comment
		if comment == "" || comment == "-" {
			printPrompt("请输入自我评价内容（Ctrl+D 结束）: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString(0)
			if err != nil && err != io.EOF {
				printError(fmt.Errorf("读取 stdin 评价内容失败: %w", err))
				return
			}
			comment = strings.TrimSpace(input)
			if comment == "" {
				printEnvelope(envelope.Error(400, "评价内容不能为空"))
				return
			}
		}

		printVerbose("正在提交自我评价...")
		err = c.SubmitSelfEvaluation(cmd.Context(), token, comment)
		if err != nil {
			printError(fmt.Errorf("提交自我评价失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("自我评价提交成功"))
	},
}

func init() {
	registerBizFlags(selfEvalSubmitCmd)
	selfEvalSubmitCmd.Flags().String("comment", "", "评价文本（空或 - 则从 stdin 读取）")
	selfEvalSubmitCmd.Flags().String("payload", "", "结构化评价 JSON（与 --comment 互斥，可用 @file.json 或 - 读取）")
}
