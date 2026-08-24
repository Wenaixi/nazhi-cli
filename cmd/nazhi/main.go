package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Wenaixi/nazhi-cli/internal/recoverx"
	"github.com/Wenaixi/nazhi-cli/internal/version"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/spf13/cobra"
)

var (
	verbose      bool
	quiet        bool
	cliLogLevel  string
	cliLogFormat string
	cliLogFile   string
)

var rootCmd = &cobra.Command{
	Use:     "nazhi",
	Short:   "nazhi -- 纳智综合评价自动化 CLI",
	Version: version.Version,
	Long: `nazhi 是纳智综合评价自动化系统的命令行工具。

	提供登录、任务管理、自我评价、文件上传等完整功能。
	所有命令输出 JSON 格式，便于脚本解析。`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		tid := logx.NewTraceID()
		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		ctx := logx.WithTraceID(parent, tid)
		cmd.SetContext(ctx)
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func main() {
	// 顶层 panic recover 契约：panic 经 printError 输出 JSON envelope 并以
	// 退出码 2（printError 默认 HTTP 500 → ExitCode=2 服务端错误档）退出，
	// 同时 debug.Stack() 写 stderr 辅助定位。
	// recover 必须在 main 顶层 defer：cobra 内部不主动 recover Run 回调 panic。
	defer func() {
		if r := recover(); r != nil {
			printError(fmt.Errorf("内部错误: %v", r))
			// 借用 recoverx.RecoverPanic 统一输出 debug.Stack()，不关心返回的 error（printError 已覆盖）
			_ = recoverx.RecoverPanic(r, nil, "main")
		}
	}()

	defer func() {
		// 关闭所有 Client（keep-alive 连接等资源）
		// 错误仅记录, 不影响 exit code (Close 失败不应改变用户感知的执行结果)
		if err := closeAllClients(); err != nil {
			printError(fmt.Errorf("关闭 Client 资源失败: %w", err))
		}
		if err := closeLogFiles(); err != nil {
			fmt.Fprintf(os.Stderr, "warn: 关闭日志文件失败: %v\n", err)
		}
	}()
	// printError 不再 os.Exit，改为设 pendingExitCode。
	// 这里把 Execute 返回 error 和 pendingExitCode 合并判断退出码。
	// 用 printError(execErr) 代替 fmt.Fprintln(os.Stderr, execErr)
	// 让 cobra parse error 走与 Run 回调相同的 JSON envelope 路径。
	// 配合 init() 里的 SilenceErrors + SilenceUsage，根除 stderr 重复输出。
	execErr := rootCmd.Execute()
	if execErr != nil {
		// cobra 返回的 execErr 来自参数解析（如 --unknownflag），本质是用户参数错误。
		// 用 code=400 获得 exit code 3，而非走 printError 的默认 500（exit code 2）。
		printEnvelope(envelope.Error(400, execErr.Error()))
	}
	if pendingExitCode.Load() != 0 {
		// os.Exit 之前显式调 closeAllClients。
		// 原代码仅靠 defer closeAllClients()，但 Go 规范明确：os.Exit 不运行
		// deferred functions。意味着任何 CLI 错误退出（pendingExitCode=1）的路径
		// 都泄漏 HTTP 连接池等资源。
		// printError（而非 _ =）确保关闭失败时用户能看到错误提示，
		// 与正常退出路径的 defer handler 行为一致。os.Exit 会跳过后续 defer，
		// 所以打印必须在 os.Exit 之前。
		if err := closeAllClients(); err != nil {
			printError(fmt.Errorf("关闭 Client 资源失败: %w", err))
		}
		if err := closeLogFiles(); err != nil {
			fmt.Fprintf(os.Stderr, "warn: 关闭日志文件失败: %v\n", err)
		}
		// 三分退出码：
		//   pendingExitCode 由 printEnvelope/printError 按 envelope.ExitCode() 设置：
		//   0 成功 / 1 partial / 业务 / 2 服务端 / 3 参数。
		//nolint:gocritic
		// defers 在正常退出路径（pendingExitCode=0）由 defer handler 处理。
		os.Exit(int(pendingExitCode.Load()))
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
	rootCmd.PersistentFlags().StringVar(&cliLogLevel, "log-level", "", "日志级别 debug/info/warn/error 默认 warn 也可通过 NAZHI_LOG_LEVEL 设置")
	rootCmd.PersistentFlags().StringVar(&cliLogFormat, "log-format", "", "日志格式 text/json 默认 text 也可通过 NAZHI_LOG_FORMAT 设置")
	rootCmd.PersistentFlags().StringVar(&cliLogFile, "log-file", "", "日志落盘路径 也可通过 NAZHI_LOG_FILE 设置")

	// 一级命令
	rootCmd.AddCommand(loginCmd)

	// session
	rootCmd.AddCommand(sessionCmd) // session parent
	sessionCmd.AddCommand(sessionActivateCmd)

	// task
	rootCmd.AddCommand(taskCmd) // task parent
	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskSubmitCmd)
	taskCmd.AddCommand(taskSubmittedCmd)
	taskCmd.AddCommand(taskDoneCmd) // submitted 别名
	taskCmd.AddCommand(taskTeacherCmd)
	taskCmd.AddCommand(taskWithdrawnCmd)
	taskCmd.AddCommand(taskPublicCmd)
	taskCmd.AddCommand(taskEditCmd)
	taskCmd.AddCommand(taskPreviewCmd)

	// self-eval
	rootCmd.AddCommand(selfEvalCmd) // self-eval parent
	selfEvalCmd.AddCommand(selfEvalSubmitCmd)
	selfEvalCmd.AddCommand(selfEvalStatusCmd)
	selfEvalCmd.AddCommand(selfEvalGradStatusCmd)
	selfEvalCmd.AddCommand(selfEvalGradSubmitCmd)

	// file
	rootCmd.AddCommand(fileCmd) // file parent
	fileCmd.AddCommand(fileUploadCmd)
	fileCmd.AddCommand(fileDownloadCmd)

	// whoami
	rootCmd.AddCommand(whoamiCmd)

	// version
	rootCmd.AddCommand(versionCmd)

	// completion
	rootCmd.AddCommand(completionCmd)

	// typical-case
	rootCmd.AddCommand(typicalCaseCmd)

	// circle
	rootCmd.AddCommand(circleCmd)
}
