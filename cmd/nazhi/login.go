package main

import (
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// loginCmd 表示 nazhi login 命令
//
//	nazhi login -u <username> -p <password> [--sso-base <url>] [--timeout <秒>]
//
// 验证码由硅基流动 Qwen3-Omni 视觉模型识别（通过 NAZHI_OCR_API_KEY 或 SILICONFLOW_API_KEY 注入）。
// v1.4.0 起 SDK 不内置任何 OCR，必须配置 API key，否则 login 直接返回 503 引导。
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "SSO 登录纳智综合评价系统",
	Long: `完成 SSO 登录全流程：InitSession → GetSchoolID → OCR 自动识别验证码 → Login。

	验证码必须配置硅基流动 Qwen3-Omni 视觉模型 API key（NAZHI_OCR_API_KEY 或 SILICONFLOW_API_KEY）。
v1.4.0 起 SDK 不再内置 ddddocr，必须通过环境变量注入视觉模型。未配置时 login 直接 503 退出。`,
	Example: `  export NAZHI_OCR_API_KEY=sk-...             # 先设置视觉模型 key
  nazhi login -u 学号 -p 密码                       # 全自动 OCR 识别验证码
	  nazhi login -u 学号 -p 密码 --sso-base https://www.nazhisoft.com --timeout 30`,
	Run: func(cmd *cobra.Command, args []string) {
		// username/password 用 applyURLFlag 统一收口
		// 语义：flag 显式传递 → 用 flag 值（含显式空字符串）；未传 → env fallback。
		// 与 client_builder.go token 读取对称。
		username := applyURLFlag(cmd, "username", "NAZHI_USERNAME")
		password := applyURLFlag(cmd, "password", "NAZHI_PASSWORD")

		if username == "" || password == "" {
			printEnvelope(envelope.Error(400, "--username 和 --password 为必填（也可通过 NAZHI_USERNAME/NAZHI_PASSWORD 环境变量设置）"))
			return
		}

		// SSO 命令（login）不要求 token，复用 buildClient 共享 env fallback。
		c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在自动识别验证码并登录...")
		resp, err := c.Login(cmd.Context(), types.LoginRequest{
			Username: username,
			Password: password,
		})
		if err != nil {
			// 用 errors.Is 精确匹配哨兵错误，按类别选择输出通道。
			switch {
			case errors.Is(err, client.ErrOCRNotConfigured) || errors.Is(err, client.ErrOCRPanic):
				printEnvelope(envelope.Error(503, "登录失败：验证码识别器未配置或出错。v1.4.0 起 SDK 不再内置 OCR，必须设置环境变量 NAZHI_OCR_API_KEY（或 SILICONFLOW_API_KEY）接入硅基流动 Qwen3-Omni 视觉模型，或通过 SDK WithCustomOCR 注入自定义识别器。"))
			case errors.Is(err, client.ErrLoginRejected):
				printEnvelope(envelope.Error(401, fmt.Sprintf("登录失败: %s（请检查学号/密码，或确认 SSO 服务端正常）", err.Error())))
			default:
				printError(fmt.Errorf("登录失败: %w", err))
			}
			return
		}
		printEnvelope(envelope.Success(resp))
	},
}

func init() {
	loginCmd.Flags().StringP("username", "u", "", "学号（必填）")
	loginCmd.Flags().StringP("password", "p", "", "密码（必填）")
	loginCmd.Flags().String("sso-base", "", "SSO 根地址（默认 https://www.nazhisoft.com）")
	loginCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
