package main

import (
	"fmt"
	"time"

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
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		ssoBase, _ := cmd.Flags().GetString("sso-base")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		if username == "" || password == "" {
			printError(fmt.Errorf("--username 和 --password 为必填"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if ssoBase != "" {
			opts = append(opts, client.WithSSOBase(ssoBase))
		}
		c := client.New(opts...)

		printVerbose("正在自动识别验证码并登录（OCR）...")
		resp, err := c.Login(cmd.Context(), types.LoginRequest{
			Username: username,
			Password: password,
		})
		if err != nil {
			printError(fmt.Errorf("登录失败: %w", err))
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
