package main

import (
	"fmt"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleLikeCmd 表示 nazhi circle like 命令
var circleLikeCmd = &cobra.Command{
	Use:     "like",
	Short:   "点赞/取消点赞写实记录",
	Long:    "给指定写实记录点赞或取消点赞。服务端自动切换点赞/取消状态。",
	Example: "  nazhi circle like --id 123456 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		idStr, _ := cmd.Flags().GetString("id")
		if idStr == "" {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			printEnvelope(envelope.Error(400, "--id 必须为正整数"))
			return
		}

		printVerbose("正在点赞...")
		if err := c.SetCircleLike(cmd.Context(), token, id); err != nil {
			printError(fmt.Errorf("点赞失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("操作成功"))
	},
}

func init() {
	circleCmd.AddCommand(circleLikeCmd)
	circleLikeCmd.Flags().String("id", "", "写实记录 ID（必填）")
	registerBizFlags(circleLikeCmd)
}
