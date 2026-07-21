package main

import (
	"fmt"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleCmd 表示 nazhi circle 父命令
var circleCmd = &cobra.Command{
	Use:   "circle",
	Short: "写实管理",
	Long:  `管理写实记录：删除、评论、点赞。`,
}

// circleDeleteCmd 表示 nazhi circle delete 命令
var circleDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "删除一条写实记录",
	Long:    "删除指定 ID 的写实记录。删除后不可恢复。",
	Example: "  nazhi circle delete --id 123456 --token xxx",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
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

		printVerbose("正在删除写实记录 id=%d...", id)
		if err := c.DeleteCircle(cmd.Context(), token, id); err != nil {
			printError(fmt.Errorf("删除写实记录失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("删除成功"))
	},
}

func init() {
	circleCmd.AddCommand(circleDeleteCmd)
	circleDeleteCmd.Flags().String("id", "", "写实记录 ID（必填）")
	registerBizFlags(circleDeleteCmd)
}
