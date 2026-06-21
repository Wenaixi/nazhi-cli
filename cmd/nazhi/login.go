package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/spf13/cobra"
)

// loginCmd 表示 nazhi login 命令
//
//	nazhi login -u <username> -p <password> [-c <captcha>] [--sso-base <url>] [--timeout <秒>]
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "SSO 登录纳智综合评价系统",
	Long: `完成 SSO 登录全流程：InitSession → GetSchoolID → 验证码 → Login。

验证码支持 3 种模式：
  1. -c/--code 提供验证码文本 — 直接使用
  2. --ocr 默认启用 — 内置 OCR 自动识别验证码（模型已内嵌，无需下载）
  3. --ocr=false 且无 -c — 交互式从 stdin 输入验证码`,
	Example: `  nazhi login -u S1234567890 -p TestPass123                       # 自动 OCR
  nazhi login -u S1234567890 -p TestPass123 -c AB12               # 指定验证码
  nazhi login -u S1234567890 -p TestPass123 --ocr=false           # 交互式输入
  nazhi login -u S1234567890 -p TestPass123 --sso-base https://www.nazhisoft.com --timeout 30`,
	Run: func(cmd *cobra.Command, args []string) {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		captchaCode, _ := cmd.Flags().GetString("code")
		ocrEnabled, _ := cmd.Flags().GetBool("ocr")
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
		if ocrEnabled {
			opts = append(opts, client.WithOCR())
		}
		c := client.New(opts...)

		// 如果未提供验证码且 OCR 未启用，进入交互流程
		if captchaCode == "" && !ocrEnabled {
			printVerbose("获取验证码中...")
			base64Img, schoolID, err := c.FetchCaptcha(cmd.Context(), username)
			if err != nil {
				printError(fmt.Errorf("获取验证码失败: %w", err))
				return
			}

			// 输出 Base64 图片到 stderr（脚本可重定向提取）
			fmt.Fprintf(os.Stderr, "Captcha Image (base64):\n%s\n\n", base64Img)

			// 从 stdin 读取验证码
			fmt.Fprint(os.Stderr, "请输入验证码: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			captchaCode = strings.TrimSpace(input)

			if captchaCode == "" {
				printError(fmt.Errorf("验证码不能为空"))
				return
			}

			// 执行登录
			printVerbose("正在登录...")
			resp, err := c.Login(cmd.Context(), types.LoginRequest{
				Username: username,
				Password: password,
				Captcha:  captchaCode,
				SchoolID: schoolID,
			})
			if err != nil {
				printError(fmt.Errorf("登录失败: %w", err))
				return
			}
			printJSON(resp)
			return
		}

		// 全自动登录（有验证码 或 OCR 自动识别）
		if ocrEnabled && captchaCode == "" {
			printVerbose("正在自动识别验证码并登录（OCR）...")
		} else if captchaCode != "" {
			printVerbose("正在登录（全自动）...")
		}
		resp, err := c.Login(cmd.Context(), types.LoginRequest{
			SchoolID: "",
			Username: username,
			Password: password,
			Captcha:  captchaCode,
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
	loginCmd.Flags().StringP("code", "c", "", "验证码文本（不传则 OCR 自动识别或交互式输入）")
	loginCmd.Flags().Bool("ocr", true, "启用内置 OCR 自动识别验证码（默认开启，模型已内嵌）")
	loginCmd.Flags().String("sso-base", "", "SSO 根地址（默认 https://www.nazhisoft.com）")
	loginCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
