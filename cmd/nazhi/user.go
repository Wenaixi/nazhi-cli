package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// userCmd 表示 nazhi user 父命令
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "用户管理",
	Long:  `管理用户信息。`,
}

// userUpdateCmd 表示 nazhi user update 命令
//
//	nazhi user update --token <token> --payload '<json>' [--base-url <url>] [--timeout <秒>]
var userUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "更新个人信息",
	Long:  `更新当前用户的个人信息。`,
	Example: `  nazhi user update --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"name":"张三","telephone":"13800138000"}'
		  nazhi user update --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @user.json`,
	Run: func(cmd *cobra.Command, args []string) {
		payloadRaw, _ := cmd.Flags().GetString("payload")
		if payloadRaw == "" {
			printEnvelope(envelope.Error(400, "--payload 为必填"))
			return
		}

		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		payloadBytes, err := parsePayloadFromArg(payloadRaw)
		if err != nil {
			printError(err)
			return
		}
		var userInfo types.UserInfo
		if err := json.Unmarshal(payloadBytes, &userInfo); err != nil {
			printError(fmt.Errorf("解析用户信息 JSON 失败: %w", err))
			return
		}

		printVerbose("正在更新个人信息...")
		if err := c.UpdateMyInfo(cmd.Context(), token, userInfo); err != nil {
			printError(fmt.Errorf("更新个人信息失败: %w", err))
			return
		}

		printEnvelope(envelope.Empty("个人信息已更新"))
	},
}

func init() {
	// user 父命令
	rootCmd.AddCommand(userCmd)

	// user update
	userCmd.AddCommand(userUpdateCmd)
	userUpdateCmd.Flags().String("payload", "", "用户信息 JSON（必填，可用 @file.json 从文件读取，或 - 从 stdin 读取）")
	registerBizFlags(userUpdateCmd)
}
