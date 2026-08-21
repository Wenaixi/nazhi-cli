package main

import (
	"fmt"
	"strconv"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleCommentCmd 表示 nazhi circle comment 命令
var circleCommentCmd = &cobra.Command{
	Use:     "comment",
	Short:   "添加写实评论",
	Long:    "给指定写实记录添加评论。",
	Example: "  nazhi circle comment --id 123456 --content '写得好' --token xxx",
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

		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			printEnvelope(envelope.Error(400, "--content 为必填"))
			return
		}

		printVerbose("正在添加评论...")
		comment, err := c.AddCircleComment(cmd.Context(), token, id, content)
		if err != nil {
			printError(fmt.Errorf("添加评论失败: %w", err))
			return
		}
		if comment != nil {
			printEnvelope(envelope.Success(comment))
			return
		}
		printEnvelope(envelope.Empty("评论成功"))
	},
}

func init() {
	circleCmd.AddCommand(circleCommentCmd)
	circleCommentCmd.Flags().String("id", "", "写实记录 ID（必填）")
	circleCommentCmd.Flags().String("content", "", "评论内容（必填）")
	registerBizFlags(circleCommentCmd)
}
