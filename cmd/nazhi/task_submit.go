package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// taskSubmitCmd 表示 nazhi task submit 命令
//
//	nazhi task submit --token <token> --payload '<json>' [--base-url <url>] [--timeout <秒>]
var taskSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交任务",
	Long:  `提交一次任务。payload 是完整的 addCircle 请求体（29 字段 JSON），可用 @file.json 从文件读取。`,
	Example: `  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"circleTaskId":1001,"circleTypeId":9256,"name":"班会","hours":1}'
	  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @task.json`,
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

		// 支持从文件读取 @file.json
		var payloadBytes []byte
		if strings.HasPrefix(payloadRaw, "@") {
			filePath := payloadRaw[1:]
			var err error
			payloadBytes, err = os.ReadFile(filePath)
			if err != nil {
				printError(fmt.Errorf("读取 payload 文件失败: %w", err))
				return
			}
		} else {
			payloadBytes = []byte(payloadRaw)
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
	taskSubmitCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	taskSubmitCmd.Flags().String("payload", "", "任务 JSON（必填，可用 @file.json 从文件读取）")
	taskSubmitCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	taskSubmitCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
