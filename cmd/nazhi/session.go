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
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("激活 Session...")
		raw, err := c.ActivateSessionJSON(cmd.Context(), token)
		if err != nil {
			printSessionActivateError(err)
			return
		}

		if len(raw) == 0 {
			printEnvelope(envelope.Empty("get_my_info_nil"))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// printSessionActivateError 是 session activate 的错误分类分支。
//
// 单独成函数是为了让测试能验证**生产代码本身**而非复制一份：
// buildBizClient 每次新建 Client，而 backoff 状态存在 Client 私有的
// sessionManager 里（新 Client 即新空 sessionManager），因此经 cobra
// 命令路径无法让 backoff 分支真实触发——测试只能构造错误直接调本函数。
// 分类逻辑此前内联在 Run 里，测试复制了一份，导致生产分支改动时测试不会红。
func printSessionActivateError(err error) {
	switch {
	case errors.Is(err, client.ErrSessionBackoff):
		printEnvelope(envelope.Partial(429, "session 激活冷却中，上次激活失败请稍后重试", nil))
	case errors.Is(err, client.ErrEmptyUserInfo):
		printEnvelope(envelope.Empty("get_my_info_empty"))
	default:
		printError(fmt.Errorf("激活 Session 失败: %w", err))
	}
}

func init() {
	sessionCmd.AddCommand(sessionActivateCmd)

	registerBizFlags(sessionActivateCmd)
}
