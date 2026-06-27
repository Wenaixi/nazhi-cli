package main

import (
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// loginCmd 表示 nazhi login 命令
//
//	nazhi login -u <username> -p <password> [--sso-base <url>] [--timeout <秒>]
//
// 验证码由内置 OCR 全自动识别，无需人工干预。
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "SSO 登录纳智综合评价系统",
	Long: `完成 SSO 登录全流程：InitSession → GetSchoolID → OCR 自动识别验证码 → Login。

	验证码由内置 OCR 全自动识别（模型已内嵌在二进制中，无需下载），无需人工干预。`,
	Example: `  nazhi login -u 学号 -p 密码                       # 全自动 OCR
	  nazhi login -u 学号 -p 密码 --sso-base https://www.nazhisoft.com --timeout 30`,
	Run: func(cmd *cobra.Command, args []string) {
		// username/password 用 applyURLFlag 统一收口
		// 语义：flag 显式传递 → 用 flag 值（含显式空字符串）；未传 → env fallback。
		// 与 client_builder.go token 读取对称。
		username := applyURLFlag(cmd, "username", "NAZHI_USERNAME")
		password := applyURLFlag(cmd, "password", "NAZHI_PASSWORD")

		if username == "" || password == "" {
			printError(fmt.Errorf("--username 和 --password 为必填（也可通过 NAZHI_USERNAME/NAZHI_PASSWORD 环境变量设置）"))
			return
		}

		// SSO 命令（login/school）不要求 token，复用 buildClient 共享 env fallback。
		c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在自动识别验证码并登录（OCR）...")
		resp, err := c.Login(cmd.Context(), types.LoginRequest{
			Username: username,
			Password: password,
		})
		if err != nil {
			// 用 errors.Is 精确匹配哨兵错误，按类别选择输出通道。
			if errors.Is(err, client.ErrOCRNotConfigured) || errors.Is(err, client.ErrOCRPanic) {
				printJSON(map[string]any{
					"status":  "error",
					"message": "登录失败：OCR 识别器未配置或出错。当前构建未启用 -tags ddddocr，请使用预编译 release 二进制，或通过 SDK 调 client.WithCustomOCR(myRecognizer) 注入识别器。",
				})
				markError()
			} else if errors.Is(err, client.ErrLoginRejected) {
				printError(fmt.Errorf("登录失败: %w（请检查学号/密码，或确认 SSO 服务端正常）", err))
			} else {
				printError(fmt.Errorf("登录失败: %w", err))
			}
			return
		}
		printJSON(resp)
	},
}

func init() {
	loginCmd.Flags().StringP("username", "u", "", "学号（必填）")
	loginCmd.Flags().StringP("password", "p", "", "密码（必填）")
	loginCmd.Flags().String("sso-base", "", "SSO 根地址（默认 https://www.nazhisoft.com）")
	loginCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
