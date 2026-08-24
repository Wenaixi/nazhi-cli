package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

var taskPreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "预览 task submit/edit 最终提交的 JSON payload（不调用提交接口）",
	Long: `预览写实提交/编辑最终发送的 JSON payload，不调用 addCircle/editCircle 接口。
注意：预览仍需联网拉取任务元数据（getCircleTypeByTaskId）并预热 session。

展示 SDK 自动补齐结果：circleTaskId/circleTypeId/dimensionId 来自任务元数据；
hours 在任务预设 >0 且用户留空时自动填充；pictureList 只含 ImageIDs
（预览路径不上传图片，ImagePaths 被忽略）。
空地址/空组织单位/空等级保持原样，不会回填学校名或默认等级「5」。
用户字段 Trim 后原样发送，与前端 JSON.stringify(form) 对齐。`,
	Example: `  nazhi task preview --token xxx --payload '{"taskId":18154,"content":"heart"}'
  nazhi task preview --token xxx --payload @task.json
  echo '{"id":5400001,"taskId":18154,"content":"fix"}' | nazhi task preview --token xxx --payload - --edit`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		isEdit, _ := cmd.Flags().GetBool("edit")
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填项"))
			return
		}
		payloadBytes, err := parseJSONObjectPayload(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}
		if isEdit {
			input, err := decodeTaskEditInput(payloadBytes)
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
			printVerbose("正在预览编辑 payload（自动补齐任务元数据，不提交）...")
			payload, err := c.PreviewEditPayload(cmd.Context(), token, input)
			if err != nil {
				printError(fmt.Errorf("预览编辑 payload 失败: %w", err))
				return
			}
			printEnvelope(envelope.Success(payload))
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
		printVerbose("正在预览提交 payload（自动补齐任务元数据，不提交）...")
		payload, err := c.PreviewSubmitPayload(cmd.Context(), token, input)
		if err != nil {
			printError(fmt.Errorf("预览提交 payload 失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(payload))
	},
}

func init() {
	registerBizFlags(taskPreviewCmd)
	taskPreviewCmd.Flags().String("payload", "", "任务 JSON（必填，@file.json 读文件或 - 读 stdin）")
	taskPreviewCmd.Flags().String("address", "", "覆盖 payload.address；留空保持为空，不回填学校名")
	taskPreviewCmd.Flags().String("level", "", "覆盖等级代码；留空保持为空，不填默认值")
	taskPreviewCmd.Flags().Bool("edit", false, "预览编辑模式（payload 必须含 id）")
}
