package main

import (
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// notificationCmd 表示 nazhi notification 父命令
var notificationCmd = &cobra.Command{
	Use:   "notification",
	Short: "通知管理",
	Long:  `查询和管理通知消息。`,
}

// notificationUnreadCmd 表示 nazhi notification unread 命令
//
//	nazhi notification unread --token <token> [--base-url <url>] [--timeout <秒>]
var notificationUnreadCmd = &cobra.Command{
	Use:   "unread",
	Short: "获取未读通知",
	Long:  `获取当前用户的未读通知列表。`,
	Example: `  nazhi notification unread --token eyJhbGciOiJIUzI1NiJ9.xxx
		  nazhi notification unread --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取未读通知...")
		notifications, err := c.GetUnreadNotifications(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取未读通知失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(notifications))
	},
}

// notificationReadCmd 表示 nazhi notification read 命令
//
//	nazhi notification read --token <token> --id <id> [--base-url <url>] [--timeout <秒>]
var notificationReadCmd = &cobra.Command{
	Use:   "read",
	Short: "标记通知为已读",
	Long:  `将指定通知标记为已读。`,
	Example: `  nazhi notification read --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123
		  nazhi notification read --token eyJhbGciOiJIUzI1NiJ9.xxx --id 123 --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		notificationID, _ := cmd.Flags().GetInt64("id")
		if notificationID == 0 {
			printEnvelope(envelope.Error(400, "--id 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在标记通知为已读...")
		if err := c.ReadNotification(cmd.Context(), token, notificationID); err != nil {
			printError(fmt.Errorf("标记通知为已读失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("通知已标记为已读"))
	},
}

func init() {
	// notification 父命令
	rootCmd.AddCommand(notificationCmd)

	// notification unread
	notificationCmd.AddCommand(notificationUnreadCmd)
	registerBizFlags(notificationUnreadCmd)

	// notification read
	notificationCmd.AddCommand(notificationReadCmd)
	notificationReadCmd.Flags().Int64("id", 0, "通知 ID（必填）")
	registerBizFlags(notificationReadCmd)
}
