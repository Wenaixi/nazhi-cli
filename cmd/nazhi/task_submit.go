package main

import (
	"github.com/spf13/cobra"
)

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
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runWriteOp(cmd, taskSubmitWriteOp, nil)
	},
}

func init() {
	registerBizFlags(taskSubmitCmd)
	taskSubmitCmd.Flags().String("payload", "", "任务 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	taskSubmitCmd.Flags().String("address", "", "地点（可选，覆盖 payload.address；空则原样，不默认学校名）")
	taskSubmitCmd.Flags().String("level", "", "等级代码（可选，写实：1=国家 2=省 3=地区/市 4=区县 5=校 6=年段；空则原样不默认 5）")
	attachAllowedKeysHelp(taskSubmitCmd, taskInputKeysAll.display(), false)
}
