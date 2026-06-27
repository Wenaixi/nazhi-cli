package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// selfEvalSubmitCmd 表示 nazhi self-eval submit 命令
//
//	nazhi self-eval submit --token <token> --comment "<评价>" [--base-url <url>] [--timeout <秒>]
var selfEvalSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "提交自我评价",
	Long:  `提交自我评价文本。如果 --comment 为空或为 "-"，则从 stdin 读取评价内容。`,
	Example: `  nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "很好的学期"
	  nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "-"`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		comment, _ := cmd.Flags().GetString("comment")

		// 从 stdin 读取评论（非 TTY 环境如 CI 直接读取，不阻塞）
		if comment == "" || comment == "-" {
			// CI 环境下 stdin 不是字符设备，直接读取而不等待用户输入
			// fmt.Fprint(os.Stderr,...) 改走 printPrompt
			// 统一交互提示通道（仅 isTerminalStdin 时输出，受 quiet 守卫）。
			printPrompt("请输入自我评价内容（Ctrl+D 结束）: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString(0) // 0 = null terminator，读到 EOF 为止
			comment = strings.TrimSpace(input)
			if comment == "" {
				printError(fmt.Errorf("评价内容不能为空"))
				return
			}
		}

		printVerbose("正在提交自我评价...")
		err = c.SubmitSelfEvaluation(cmd.Context(), token, comment)
		if err != nil {
			printError(fmt.Errorf("提交自我评价失败: %w", err))
			return
		}

		printJSON(map[string]string{"status": "ok", "message": "自我评价提交成功"})
	},
}

func init() {
	registerBizFlags(selfEvalSubmitCmd)
	selfEvalSubmitCmd.Flags().String("comment", "", "评价文本（空或 - 则从 stdin 读取）")
}
