package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// sessionCmd 表示 nazhi session 父命令（实际操作在 activate 子命令）。
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "管理业务 Session",
	Long:  `初始化目标平台业务 Session。后续所有业务接口调用前必须先激活，否则服务端返回空数据。`,
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
			printParamError(err)
			return
		}

		printVerbose("激活 Session...")
		raw, err := c.ActivateSessionJSON(cmd.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, client.ErrSessionBackoff):
				printEnvelope(envelope.Partial(429, "session 激活冷却中，上次激活失败请稍后重试", nil))
			case errors.Is(err, client.ErrEmptyUserInfo):
				printEnvelope(envelope.Empty("get_my_info_empty"))
			default:
				printError(fmt.Errorf("激活 Session 失败: %w", err))
			}
			return
		}

		if len(raw) == 0 {
			printEnvelope(envelope.Empty("get_my_info_nil"))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	sessionCmd.AddCommand(sessionActivateCmd)

	registerBizFlags(sessionActivateCmd)
}
