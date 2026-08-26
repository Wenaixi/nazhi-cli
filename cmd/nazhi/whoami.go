package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// whoamiCmd 表示 nazhi whoami 命令
//
//	nazhi whoami --token <token> [--base-url <url>] [--timeout <秒>]
//
// envelope.data 为经 SDK 后处理的 UserInfo 重序列化 JSON：className 去首个「级」字、
// 学校信息降级补全、未建模字段与零值字段（omitempty）不出现——并非平台原始字节；
// 需要平台原文请走业务抓包。
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "获取当前登录用户完整信息",
	Long:  `获取用户的完整个人资料，包括姓名、性别、学号、学校、年级、班级、座号等。`,
	Example: `  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在获取用户信息...")
		raw, err := c.GetMyInfoJSON(cmd.Context(), token)
		if err != nil {
			if errors.Is(err, client.ErrEmptyUserInfo) {
				printEnvelope(envelope.Empty("get_my_info_empty"))
				return
			}
			printError(fmt.Errorf("获取用户信息失败: %w", err))
			return
		}
		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	registerBizFlags(whoamiCmd)
}
