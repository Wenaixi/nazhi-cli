package main

import (
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
		// username/password 用 flagChanged() 守卫
		// env fallback，避免用户显式传 --username "" 时 NAZHI_USERNAME 静默覆盖。
		// 与 client_builder.go token 读取对称
		//   - Changed=true → 用户显式传过 flag，flag 值生效（含显式空字符串）
		//   - Changed=false → 未传 flag，走 env fallback
		var username, password string
		if flagChanged(cmd, "username") {
			username, _ = cmd.Flags().GetString("username")
		} else {
			username = envString("NAZHI_USERNAME", "")
		}
		if flagChanged(cmd, "password") {
			password, _ = cmd.Flags().GetString("password")
		} else {
			password = envString("NAZHI_PASSWORD", "")
		}

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
			// 用 ErrorCategory 分类替代 errors.Is 逐一枚举。
			switch client.ClassifyError(err) {
			case client.ErrorCategoryOCR:
				printVerbose("OCR 识别器未配置：当前构建无 -tags ddddocr。请使用预编译 release 二进制（nazhi-cli releases 页面），或通过 SDK 调 client.WithCustomOCR(myRecognizer) 注入识别器")
				printJSON(map[string]any{
					"status":  "error",
					"message": "登录失败：OCR 识别器未配置。当前构建未启用 -tags ddddocr，无法自动识别验证码。请下载预编译 release 二进制或注入自定义识别器。",
				})
				markError()
			case client.ErrorCategoryAuth:
				printError(fmt.Errorf("登录失败: %w（SSO 重定向 Location 头畸形，请检查 SSO 服务端响应或上报 bug）", err))
			default:
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
