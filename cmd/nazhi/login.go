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
// 验证码由硅基流动 Qwen3-Omni 视觉模型识别（通过 NAZHI_SILICONFLOW_API_KEY 注入，兼容旧别名）。
// SDK 不内置验证码识别器，必须配置视觉模型或注入自定义识别器，否则 login 直接返回 503 引导。
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "SSO 登录纳智综合评价系统",
	Long: `完成 SSO 登录全流程：InitSession → GetSchoolID → 视觉识别器处理验证码 → Login。

	验证码必须配置 Nazhi-auto 同款硅基流动 Qwen3-Omni API key（NAZHI_SILICONFLOW_API_KEY）。
兼容 NAZHI_OCR_API_KEY / SILICONFLOW_API_KEY 旧别名。
可选通过 NAZHI_OCR_BASE_URL / NAZHI_OCR_MODEL 覆盖识别端点与模型名（默认走硅基流动官方地址与 Qwen3-Omni）。SDK 不内置本地验证码识别器，未配置时 login 直接返回错误退出。`,
	Example: `  export NAZHI_SILICONFLOW_API_KEY=sk-...      # 先设置视觉模型 key
  nazhi login -u 学号 -p 密码                       # 视觉识别器自动处理验证码
  nazhi login -u 学号 -p 密码 --sso-base https://www.nazhisoft.com --timeout 30`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// username/password 用 applyURLFlag 统一收口：
		// flag 显式传递 → 用 flag 值（含显式空字符串）；未传 → env fallback。
		// 与 assembly.go 的 token 读取语义对称；登录专用，不走 buildClientOpts
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

		printVerbose("正在调用视觉识别器处理验证码并登录...")
		resp, err := c.Login(cmd.Context(), types.LoginRequest{
			Username: username,
			Password: password,
		})
		if err != nil {
			// 用 errors.Is 精确匹配哨兵错误，按类别选择输出通道。
			switch {
			case errors.Is(err, client.ErrOCRNotConfigured) || errors.Is(err, client.ErrOCRPanic):
				printEnvelope(envelope.Error(503, "登录失败：验证码识别器未配置或出错。请设置 Nazhi-auto 同款环境变量 NAZHI_SILICONFLOW_API_KEY 接入硅基流动 Qwen3-Omni 视觉模型（兼容 NAZHI_OCR_API_KEY / SILICONFLOW_API_KEY），或通过 SDK WithCustomOCR 注入自定义识别器。"))
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
