package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// parsePayload 解析 --payload 参数，委托公共 helper 处理 @file.json / - / 原始字符串。
func parsePayload(raw string) ([]byte, error) {
	return parsePayloadFromArg(raw)
}

// taskSubmitCmd 表示 nazhi task submit 命令
//
//	nazhi task submit --token <token> --payload '<json>' [--base-url <url>] [--timeout <秒>]
var taskSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交任务",
	Long:  `提交一次任务。payload 是完整的 addCircle 请求体（29 字段 JSON），可用 @file.json 从文件读取，或 - 从 stdin 读取。`,
	Example: `  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"circleTaskId":1001,"circleTypeId":9256,"name":"班会","hours":1}'
		  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @task.json
		  echo '{"circleTaskId":1001,"name":"班会","hours":1}' | nazhi task submit --token "xxx" --payload -`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}
		if payloadRaw == "" {
			printError(fmt.Errorf("--payload 为必填"))
			return
		}

		payloadBytes, err := parsePayload(payloadRaw)
		if err != nil {
			printError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		var payload types.TaskSubmitPayload
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			printError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		printVerbose("正在提交任务...")
		result, err := c.SubmitTask(cmd.Context(), token, payload)
		if err != nil {
			printError(fmt.Errorf("提交任务失败: %w", err))
			return
		}

		printJSON(result)
	},
}

func init() {
	registerBizFlags(taskSubmitCmd)
	taskSubmitCmd.Flags().String("payload", "", "任务 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
}
