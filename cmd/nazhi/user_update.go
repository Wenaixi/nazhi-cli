package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// userCmd 表示 nazhi user 父命令
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "用户信息管理",
	Long:  `查看和更新用户个人信息。`,
}

// userUpdateCmd 表示 nazhi user update 命令
var userUpdateCmd = &cobra.Command{
	Use:     "update",
	Short:   "更新个人信息",
	Long:    `更新当前用户的个人信息。payload 可用 @file.json 读取或 - 从 stdin 读取。`,
	Example: `  nazhi user update --token xxx --payload '{"telephone":"13800138000","familyAddress":"福建省福州市"}'`,
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		payloadBytes, err := parsePayloadFromArg(payloadRaw)
		if err != nil {
			printError(fmt.Errorf("读取 payload 失败: %w", err))
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			printError(fmt.Errorf("解析 payload JSON 失败: %w", err))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在更新个人信息...")
		if err := c.UpdateMyInfo(cmd.Context(), token, payload); err != nil {
			printError(fmt.Errorf("更新个人信息失败: %w", err))
			return
		}
		printEnvelope(envelope.Empty("个人信息更新成功"))
	},
}

// userInfoCmd 表示 nazhi user info 命令（whoami 别名）
var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "查看个人信息（whoami 别名）",
	Run:   whoamiCmd.Run,
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userUpdateCmd)
	userCmd.AddCommand(userInfoCmd)

	userUpdateCmd.Flags().String("payload", "", "用户信息 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(userUpdateCmd)
	registerBizFlags(userInfoCmd)
}
