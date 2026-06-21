package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
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
		token, _ := cmd.Flags().GetString("token")
		comment, _ := cmd.Flags().GetString("comment")
		baseURL, _ := cmd.Flags().GetString("base-url")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		if token == "" {
			printError(fmt.Errorf("--token 为必填"))
			return
		}

		// 从 stdin 读取评论
		if comment == "" || comment == "-" {
			fmt.Fprint(os.Stderr, "请输入自我评价内容（Ctrl+D 结束）: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			comment = strings.TrimSpace(input)
			if comment == "" {
				printError(fmt.Errorf("评价内容不能为空"))
				return
			}
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if baseURL != "" {
			opts = append(opts, client.WithBaseURL(baseURL))
		}
		c := client.New(opts...)

		printVerbose("正在提交自我评价...")
		err := c.SubmitSelfEvaluation(cmd.Context(), token, comment)
		if err != nil {
			printError(fmt.Errorf("提交自我评价失败: %w", err))
			return
		}

		printJSON(map[string]string{"status": "ok", "message": "自我评价提交成功"})
	},
}

func init() {
	selfEvalSubmitCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	selfEvalSubmitCmd.Flags().String("comment", "", "评价文本（空或 - 则从 stdin 读取）")
	selfEvalSubmitCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	selfEvalSubmitCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
