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
	Long:  `查看系统通知和公告。`,
}

// notificationListCmd 表示 nazhi notification list 命令
var notificationListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取通知列表",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		pageNo, _ := cmd.Flags().GetInt("page")
		pageSize, _ := cmd.Flags().GetInt("page-size")

		printVerbose("正在获取通知列表...")
		result, err := c.GetAllNotifications(cmd.Context(), token, pageNo, pageSize)
		if err != nil {
			printError(fmt.Errorf("获取通知列表失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(result))
	},
}

// notificationGetCmd 表示 nazhi notification get 命令
var notificationGetCmd = &cobra.Command{
	Use:   "get",
	Short: "查看通知详情",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		id, _ := cmd.Flags().GetInt64("id")
		if id <= 0 {
			printEnvelope(envelope.Error(400, "--id 为必填且必须 > 0"))
			return
		}

		// 自动标记为已读
		printVerbose("正在获取通知详情...")
		notification, err := c.GetNotificationByID(cmd.Context(), token, id)
		if err != nil {
			printError(fmt.Errorf("获取通知详情失败: %w", err))
			return
		}
		_ = c.ReadNotification(cmd.Context(), token, id)

		printEnvelope(envelope.Success(notification))
	},
}

// notificationUnreadCmd 表示 nazhi notification unread 命令
var notificationUnreadCmd = &cobra.Command{
	Use:   "unread",
	Short: "查看未读通知数",
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取未读通知数...")
		count, err := c.GetUnreadNotificationsCount(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取未读通知数失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(map[string]int{"unread": count}))
	},
}

func init() {
	rootCmd.AddCommand(notificationCmd)
	notificationCmd.AddCommand(notificationListCmd)
	notificationCmd.AddCommand(notificationGetCmd)
	notificationCmd.AddCommand(notificationUnreadCmd)

	notificationListCmd.Flags().Int("page", 1, "页码（从 1 开始）")
	notificationListCmd.Flags().Int("page-size", 10, "每页条数")
	registerBizFlags(notificationListCmd)

	notificationGetCmd.Flags().Int64("id", 0, "通知 ID（必填）")
	registerBizFlags(notificationGetCmd)

	registerBizFlags(notificationUnreadCmd)
}
