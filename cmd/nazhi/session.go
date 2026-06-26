package main

import (
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// sessionCmd 表示 nazhi session activate 命令
//
//	nazhi session activate --token <token> [--base-url <url>] [--timeout <秒>]
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "管理业务 Session",
	Long:  `初始化目标平台业务 Session。必须先 GET / + GET /api/studentInfo/getMenu，否则后续接口返回空数据。`,
}

var sessionActivateCmd = &cobra.Command{
	Use:   "activate",
	Short: "激活业务 Session",
	Long:  `使用 token 激活目标平台业务 Session。返回用户基本信息。`,
	Example: `  nazhi session activate --token eyJhbGciOiJIUzI1NiJ9.xxx
	  nazhi session activate --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("激活 Session...")
		info, err := c.ActivateSession(cmd.Context(), token)
		if err != nil {
			// F10 修复（round-7）：ErrEmptyUserInfo 是「业务成功但无数据」状态
			//（非错误），与 whoami 对称输出 status envelope 而非裸 null。
			//
			// 失败场景（修复前）：printJSON(info) → 输出 `null\n`
			// 与 whoami 的 {status: empty, reason: ...} 不一致。
			if errors.Is(err, client.ErrEmptyUserInfo) {
				printJSON(map[string]string{
					"status": "empty",
					"reason": "get_my_info_empty",
				})
				return
			}
			printError(fmt.Errorf("激活 Session 失败: %w", err))
			return
		}

		printJSON(info)
	},
}

func init() {
	sessionCmd.AddCommand(sessionActivateCmd)

	sessionActivateCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	sessionActivateCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	sessionActivateCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
