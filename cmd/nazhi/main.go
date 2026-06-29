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
	// 顶层 panic recover 走统一 exit code 1 契约。
	// 原代码没有 panic recover：cobra Run 回调（cmd.Run func）panic 时
	// Go runtime 直接打 stack trace + exit code 2，违反 F7 设计的「统一
	// exit code 1」契约。CI 脚本区分「用户错误」(exit 1) 与「程序 bug」
	// (exit 2) 时被误导。
	// 设计契约
	//   - panic 发生 → recover
	//   - pendingExitCode 标记为 1（与正常 error 路径一致）
	//   - 不打 stack trace 给终端用户（避免噪声 + 信息泄露）
	//   - 后续 closeAllClients() 仍跑（defer 在 panic 后也会运行，但 os.Exit 不会）
	//   - 最终 os.Exit(1) 走与正常错误相同的退出码
	// 注意：recover 必须在 main 顶层 defer，否则 panic 会跨过 rootCmd.Execute()
	// 直接打到 Go runtime。Cobra 内部不主动 recover Run 回调 panic。
	// F4: recover handler 不再直接 os.Exit(1)，而是 printError 设 pendingExitCode=1，
	// 让执行流 fall through 到 line 77 的 pendingExitCode 统一出口。
	// LIFO 顺序下 closeAllClients 的 defer（行 64-69）在处理本函数前
	// 已运行释放资源，行 85 的 closeAllClients 幂等安全。
	defer func() {
		if r := recover(); r != nil {
			// F9: 把 panic 转成 printError 输出，与正常 error 路径一致
			// 不打 stack trace 给终端用户（生产 CLI 应当简洁）
			printError(fmt.Errorf("内部错误: %v", r))
		}
	}()

	defer func() {
		// 关闭所有 Client (ONNX session + 临时目录 + keep-alive 连接)
		// 错误仅记录, 不影响 exit code (Close 失败不应改变用户感知的执行结果)
		if err := closeAllClients(); err != nil {
			printError(fmt.Errorf("关闭 Client 资源失败: %w", err))
		}
	}()
	// printError 不再 os.Exit，改为设 pendingExitCode。
	// 这里把 Execute 返回 error 和 pendingExitCode 合并判断退出码。
	// 用 printError(execErr) 代替 fmt.Fprintln(os.Stderr, execErr)
	// 让 cobra parse error 走与 Run 回调相同的 JSON envelope 路径。
	// 配合 init() 里的 SilenceErrors + SilenceUsage，根除 stderr 重复输出。
	execErr := rootCmd.Execute()
	if execErr != nil {
		printError(execErr)
	}
	if pendingExitCode.Load() != 0 {
		// os.Exit 之前显式调 closeAllClients。
		// 原代码仅靠 defer closeAllClients()，但 Go 规范明确：os.Exit 不运行
		// deferred functions。意味着任何 CLI 错误退出（pendingExitCode=1）的路径
		// 都泄漏 ONNX session + tempDir + keep-alive 连接。
		// 修复：os.Exit 前显式 closeAllClients()。幂等安全：closeAllClients 内
		// 部把全局 pendingClients 置 nil，二次调用是 no-op，defer 再跑也不会出错。
		_ = closeAllClients()
		os.Exit(1)
	}
}

func init() {
	// 静音 cobra 默认的错误打印与 usage 打印。
	// 让 main.go 用 printError(execErr) 单一来源输出错误
	// 避免用户看到 "Error: ..." + Usage + 另一遍 "unknown flag" 的重复。
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	// 全局标志
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细日志输出到 stderr")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "静默模式，关闭所有 stderr 输出")

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
