package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// parsePayload 解析 --payload 参数，委托公共 helper 处理 @file.json / - / 原始字符串。
func parsePayload(raw string) ([]byte, error) {
	return parsePayloadFromArg(raw)
}

// taskSubmitCmd 表示 nazhi task submit 命令。
//
// 公开输入：taskId / content / imagePaths 等用户字段；SDK 只自动补任务元数据与图片上传。
// address/level 为空时原样提交，不再默认学校名或等级 5。
var taskSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交任务",
	Long:  `提交一次任务。payload 是最小必要输入 JSON，可用 @file.json 从文件读取，或 - 从 stdin 读取。SDK 自动补齐任务元数据、图片上传结果并提交；address/level 等用户字段空串原样发送。`,
	Example: `  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"taskId":18154,"content":"劳动让我体会到责任的重要性。"}'
		  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"taskId":18154,"content":"劳动让我体会到责任的重要性。","imagePaths":["./photo.jpg"],"playRole":"3"}'
		  nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @task.json --address "操场" --level 5
		  echo '{"taskId":18154,"content":"劳动让我体会到责任的重要性。"}' | nazhi task submit --token "xxx" --payload -`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		payloadBytes, err := parsePayload(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		input, err := decodeTaskSubmitInput(payloadBytes)
		if err != nil {
			printParamError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		if v, _ := cmd.Flags().GetString("address"); v != "" {
			input.Address = v
		}
		if v, _ := cmd.Flags().GetString("level"); v != "" {
			input.Level = v
		}

		printVerbose("正在提交任务（自动补全任务元数据/图片上传）...")
		result, err := c.SubmitTask(cmd.Context(), token, input)
		if err != nil {
			printError(fmt.Errorf("提交任务失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(result))
	},
}

func init() {
	registerBizFlags(taskSubmitCmd)
	taskSubmitCmd.Flags().String("payload", "", "任务 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	taskSubmitCmd.Flags().String("address", "", "地点（可选，覆盖 payload.address；空则原样，不默认学校名）")
	taskSubmitCmd.Flags().String("level", "", "等级代码（可选，写实：1=国家 2=省 3=地区/市 4=区县 5=校 6=年段；空则原样不默认 5）")
}
