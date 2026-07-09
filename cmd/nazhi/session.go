package main

import (
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
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
			// 用 errors.Is 精确匹配哨兵错误。
			switch {
			case errors.Is(err, client.ErrSessionBackoff):
				// ErrSessionBackoff 在冷却窗口内被抑制
				// 输出 partial envelope（友好 cooldown 提示），code=429（业务节流）
				printEnvelope(envelope.Partial(429, "session 激活冷却中，上次激活失败请稍后重试", nil))
			case errors.Is(err, client.ErrEmptyUserInfo):
				// ErrEmptyUserInfo 是「业务成功但无数据」状态
				printEnvelope(envelope.Empty("get_my_info_empty"))
			default:
				printError(fmt.Errorf("激活 Session 失败: %w", err))
			}
			return
		}

		if info == nil {
			printEnvelope(envelope.Empty("get_my_info_nil"))
			return
		}

		printEnvelope(envelope.Success(info))
	},
}

func init() {
	sessionCmd.AddCommand(sessionActivateCmd)

	registerBizFlags(sessionActivateCmd)
}
