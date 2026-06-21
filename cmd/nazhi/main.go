// cmd/nazhi/main.go — nazhi CLI 入口 (cobra root)
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	quiet   bool
	output  string
)

var rootCmd = &cobra.Command{
	Use:   "nazhi",
	Short: "nazhi — 纳智综合评价自动化 CLI",
	Long: `nazhi 是纳智综合评价自动化系统的命令行工具。

提供登录、任务管理、自我评价、文件上传等完整功能。
所有命令输出 JSON 格式，便于脚本解析。`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// 全局标志
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细日志输出到 stderr")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "静默模式，关闭所有 stderr 输出")
	rootCmd.PersistentFlags().StringVar(&output, "output", "json", "输出格式（默认 json）")

	// 一级命令
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(schoolCmd)

	// session
	rootCmd.AddCommand(sessionCmd)       // session parent
	sessionCmd.AddCommand(sessionActivateCmd)

	// task
	rootCmd.AddCommand(taskCmd)          // task parent
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskSubmitCmd)

	// self-eval
	rootCmd.AddCommand(selfEvalCmd)       // self-eval parent
	selfEvalCmd.AddCommand(selfEvalSubmitCmd)
	selfEvalCmd.AddCommand(selfEvalStatusCmd)

	// file
	rootCmd.AddCommand(fileCmd)          // file parent
	fileCmd.AddCommand(fileUploadCmd)
}
