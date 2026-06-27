package main

import (
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// whoamiCmd 表示 nazhi whoami 命令
//
//	nazhi whoami --token <token> [--base-url <url>] [--timeout <秒>]
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "获取当前登录用户完整信息",
	Long:  `获取用户的完整个人资料，包括姓名、性别、学号、学校、年级、班级、座号等。`,
	Example: `  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx
	  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取用户信息...")
		info, err := c.GetMyInfo(cmd.Context(), token)
		if err != nil {
			// ErrEmptyUserInfo 是「业务成功但无数据」状态
			//（非错误），按 status envelope 输出而非走 printError。
			// 修复前：GetMyInfo 返回 (nil, nil) → info == nil → 输出 status envelope
			//。但 SDK 现在改返回 (nil, ErrEmptyUserInfo) →
			// 不再是 (nil,nil)，info 仍 nil 但 err 非 nil，必须用 errors.Is 分支。
			// 与 session activate 契约对称：两个 cmd 在「业务成功无数据」时
			// 都输出 {status: empty, reason: get_my_info_empty}。
			if errors.Is(err, client.ErrEmptyUserInfo) {
				printJSON(map[string]string{
					"status": "empty",
					"reason": "get_my_info_empty",
				})
				return
			}
			printError(fmt.Errorf("获取用户信息失败: %w", err))
			return
		}
		// info 非 nil 但 err 为 nil：正常路径
		printJSON(info)
	},
}

func init() {
	registerBizFlags(whoamiCmd)
}
