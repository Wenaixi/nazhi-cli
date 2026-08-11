package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskEditCmd 表示 nazhi task edit 命令
//
//	nazhi task edit --token <token> --payload '<json>'
//	nazhi task edit --token <token> --payload @edit.json
//	echo '{"id":5464109,"taskId":18151,"content":"修改后的内容"}' | nazhi task edit --token "xxx" --payload -
//
// 修改已提交但未审核的写实记录。payload 是最小必要输入 JSON，
// 可用 @file.json 从文件读取，或 - 从 stdin 读取。
var taskEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "修改已提交的写实记录",
	Long: `修改一条已提交但未审核的写实记录。

调用 editCircle 接口，SDK 内部自动完成 getCircleTypeByTaskId 元数据预取、
图片上传、字段补齐等流程。

必填字段：id（写实记录主键）、taskId（任务 ID）、content（心得/感悟）。
可选字段与 task submit 完全一致：imagePaths / imageIDs / playRole / address / level 等。`,
	Example: `  nazhi task edit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"id":5464109,"taskId":18151,"content":"修改后的内容"}'
  nazhi task edit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @edit.json
  echo '{"id":5464109,"taskId":18151,"content":"修改后的内容"}' | nazhi task edit --token "xxx" --payload -`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		payloadBytes, err := parsePayloadFromArg(payloadRaw)
		if err != nil {
			printParamError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}

		input, err := decodeTaskEditInput(payloadBytes)
		if err != nil {
			printParamError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		// 支持 --address / --level 独立 flag 覆盖 payload 中的值
		if v, _ := cmd.Flags().GetString("address"); v != "" {
			input.Address = v
		}
		if v, _ := cmd.Flags().GetString("level"); v != "" {
			input.Level = v
		}

		printVerbose("正在修改写实记录（自动补全任务元数据/图片上传）...")
		result, err := c.EditCircle(cmd.Context(), token, input)
		if err != nil {
			printError(fmt.Errorf("修改写实记录失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(result))
	},
}

func init() {
	registerBizFlags(taskEditCmd)
	taskEditCmd.Flags().String("payload", "", "修改写实记录的 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	taskEditCmd.Flags().String("address", "", "地点（可选，覆盖 payload.address；空则原样，不默认学校名）")
	taskEditCmd.Flags().String("level", "", "等级代码（可选，写实：1=国家 2=省 3=地区/市 4=区县 5=校 6=年段；空则原样不默认 5）")
}
