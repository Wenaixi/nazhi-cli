package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// loginCmd 表示 nazhi login 命令
//
//	nazhi login -u <username> -p <password> [--sso-base <url>] [--timeout <秒>]
//
// 免验证码：直接调用免验证码登录端点（/uiActivityLogin/studentLogin），无需
// 外部 OCR 或验证码配置。
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "SSO 登录纳智综合评价系统",
	Long: `完成 SSO 登录全流程：InitSession → GetSchoolID → 免验证码登录（/uiActivityLogin/studentLogin）。

	免验证码端点直接登录，无需任何外部 OCR 或验证码配置。
	可选通过 --sso-base 指定 SSO 根地址（默认 https://www.nazhisoft.com）。`,
	Example: `  nazhi login -u 学号 -p 密码                       # 免验证码直登
  nazhi login -u 学号 -p 密码 --sso-base https://www.nazhisoft.com --timeout 30`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// username/password 用 applyURLFlag 统一收口：
		// flag 显式传递 → 用 flag 值（含显式空字符串）；未传 → env fallback。
		// 与 assembly.go 的 token 读取语义对称；登录专用，不走 buildClientOpts
		username := applyURLFlag(cmd, "username", "NAZHI_USERNAME")
		password := applyURLFlag(cmd, "password", "NAZHI_PASSWORD")

		// 空白串用户名/密码校验——纯空格字符串在 trim 后为空，
		// 直接拒绝避免原样上送 SSO（与 task submit/edit 的 trim 校验同族）。
		if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
			printParamError(errors.New("--username 和 --password 为必填（也可通过 NAZHI_USERNAME/NAZHI_PASSWORD 环境变量设置）"))
			return
		}

		// SSO 命令（login）不要求 token，复用 buildClient 共享 env fallback。
		c, err := buildClient(cmd, "sso", "NAZHI_TIMEOUT")
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在免验证码登录...")
		resp, err := c.Login(cmd.Context(), types.LoginRequest{
			Username: username,
			Password: password,
		})
		if err != nil {
			// 全部登录失败分支写 stderr：stdout 只承载成功数据。保留各自
			// HTTP code（429/502/401）以维持退出码语义，仅统一输出通道。
			// 用 errors.Is 精确匹配哨兵错误，按类别选择文案。
			// SDK 层 auth.go:188 对非 200/302 用 classifyHTTPStatus 分类，429/5xx 错误链
			// 只含 ErrRateLimited/ErrServiceUnavailable 不含 ErrLoginRejected（外层 switch
			// 的内层分支是死代码）。改为直接按哨兵匹配，专属中文文案真正可达。
			switch {
			case errors.Is(err, client.ErrRateLimited):
				printErrorWithCode(fmt.Errorf("登录失败：请求被限流，请稍后退避重试（%s）", err.Error()), 429)
			case errors.Is(err, client.ErrServiceUnavailable):
				printErrorWithCode(fmt.Errorf("登录失败：SSO 服务端暂时不可用，请稍后重试（%s）", err.Error()), 502)
			case errors.Is(err, client.ErrLoginRejected):
				printErrorWithCode(fmt.Errorf("登录失败: %s（请检查学号/密码，或确认 SSO 服务端正常）", err.Error()), 401)
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
