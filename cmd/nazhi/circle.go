package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// circleCmd 表示 nazhi circle 父命令
var circleCmd = &cobra.Command{
	Use:   "circle",
	Short: "写实管理",
	Long:  `管理写实记录：删除写实、添加评论、点赞等。`,
}

// circleDeleteCmd 表示 nazhi circle delete 命令
//
//	nazhi circle delete --token <token> --id <id> [--base-url <url>] [--timeout <秒>]
var circleDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除写实记录",
	Long:  `按 ID 删除一条写实记录。`,
	Example: `  nazhi circle delete --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123
		  nazhi circle delete --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123 --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		circleID, _ := cmd.Flags().GetInt64("id")
		if circleID == 0 {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在删除写实记录...")
		if err := c.DeleteCircle(cmd.Context(), token, circleID); err != nil {
			printError(fmt.Errorf("删除写实记录失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("写实记录已删除"))
	},
}

// circleCommentCmd 表示 nazhi circle comment 命令
//
//	nazhi circle comment --token <token> --id <id> --content <content> [--base-url <url>] [--timeout <秒>]
var circleCommentCmd = &cobra.Command{
	Use:   "comment",
	Short: "添加写实评论",
	Long:  `为指定写实记录添加评论。`,
	Example: `  nazhi circle comment --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123 --content "写得很好！"
		  nazhi circle comment --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123 --content "写得很好！" --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		circleID, _ := cmd.Flags().GetInt64("id")
		content, _ := cmd.Flags().GetString("content")
		if circleID == 0 {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}
		if content == "" {
			printEnvelope(envelope.Error(400, "--content 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在添加评论...")
		if err := c.AddCircleComment(cmd.Context(), token, circleID, content); err != nil {
			printError(fmt.Errorf("添加评论失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("评论已添加"))
	},
}

// circleLikeCmd 表示 nazhi circle like 命令
//
//	nazhi circle like --token <token> --id <id> [--base-url <url>] [--timeout <秒>]
var circleLikeCmd = &cobra.Command{
	Use:   "like",
	Short: "点赞写实记录",
	Long:  `为指定写实记录点赞（或取消点赞）。`,
	Example: `  nazhi circle like --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123
		  nazhi circle like --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123 --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		circleID, _ := cmd.Flags().GetInt64("id")
		if circleID == 0 {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在点赞...")
		if err := c.SetCircleLike(cmd.Context(), token, circleID); err != nil {
			printError(fmt.Errorf("点赞失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("点赞成功"))
	},
}

func init() {
	// circle 父命令
	rootCmd.AddCommand(circleCmd)

	// circle delete
	circleCmd.AddCommand(circleDeleteCmd)
	circleDeleteCmd.Flags().Int64("id", 0, "写实记录 ID（必填）")
	registerBizFlags(circleDeleteCmd)

	// circle comment
	circleCmd.AddCommand(circleCommentCmd)
	circleCommentCmd.Flags().Int64("id", 0, "写实记录 ID（必填）")
	circleCommentCmd.Flags().String("content", "", "评论内容（必填）")
	registerBizFlags(circleCommentCmd)

	// circle like
	circleCmd.AddCommand(circleLikeCmd)
	circleLikeCmd.Flags().Int64("id", 0, "写实记录 ID（必填）")
	registerBizFlags(circleLikeCmd)
}
