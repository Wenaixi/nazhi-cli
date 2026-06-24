package main

import (
	"fmt"
	"os"

	"github.com/Wenaixi/nazhi-cli/internal/version"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	quiet   bool
	output  string
)

var rootCmd = &cobra.Command{
	Use:     "nazhi",
	Short:   "nazhi — 纳智综合评价自动化 CLI",
	Version: version.Version,
	Long: `nazhi 是纳智综合评价自动化系统的命令行工具。

提供登录、任务管理、自我评价、文件上传等完整功能。
所有命令输出 JSON 格式，便于脚本解析。`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func main() {
	defer func() {
		// 关闭所有 Client (ONNX session + 临时目录 + keep-alive 连接)
		// 错误仅记录, 不影响 exit code (Close 失败不应改变用户感知的执行结果)
		if err := closeAllClients(); err != nil {
			fmt.Fprintln(os.Stderr, "警告: 关闭 Client 资源失败:", err)
		}
	}()
	// F7 修复：printError 不再 os.Exit，改为设 pendingExitCode。
	// 这里把 Execute 返回 error 和 pendingExitCode 合并判断退出码。
	execErr := rootCmd.Execute()
	if execErr != nil {
		fmt.Fprintln(os.Stderr, execErr)
		markError()
	}
	if pendingExitCode.Load() != 0 {
		os.Exit(1)
	}
}

func init() {
	// 全局标志
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细日志输出到 stderr")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "静默模式，关闭所有 stderr 输出")
	// --output 当前仅支持 json（未来扩展 yaml/text 时再实现）
	rootCmd.PersistentFlags().StringVar(&output, "output", "json", "输出格式（当前仅支持 json）")
	rootCmd.PersistentFlags().Lookup("output").NoOptDefVal = "json"

	// 一级命令
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(schoolCmd)

	// session
	rootCmd.AddCommand(sessionCmd) // session parent
	sessionCmd.AddCommand(sessionActivateCmd)

	// task
	rootCmd.AddCommand(taskCmd) // task parent
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskSubmitCmd)

	// self-eval
	rootCmd.AddCommand(selfEvalCmd) // self-eval parent
	selfEvalCmd.AddCommand(selfEvalSubmitCmd)
	selfEvalCmd.AddCommand(selfEvalStatusCmd)

	// file
	rootCmd.AddCommand(fileCmd) // file parent
	fileCmd.AddCommand(fileUploadCmd)

	// whoami
	rootCmd.AddCommand(whoamiCmd)

	// version
	rootCmd.AddCommand(versionCmd)

	// completion
	rootCmd.AddCommand(completionCmd)
}
