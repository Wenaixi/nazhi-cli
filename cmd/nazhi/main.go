package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		// --quiet 经 recoverx.SetQuiet 传播到 pkg/client recover 路径
		// （fetchTasksForDimensionSafe 及其调用的 fetchTasksForDimension 无法感知 CLI flag）。
		recoverx.SetQuiet(quiet)
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
	// SIGINT/SIGTERM → context 取消。cobra Execute 用 rootCmd.Context，
	// PersistentPreRun 从 cmd.Context() 派生——信号到达后 Run 回调内的网络
	// 调用（含 --payload - 的 stdin 挂起）能感知 ctx.Done 提前中止，而不是
	// Go 默认直接杀进程跳过 closeAllClients（keep-alive 泄漏）。
	// 根 ctx 在 rootCmd.SetContext 注册，子命令 Run 经 cmd.Context() 继承。
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rootCmd.SetContext(sigCtx)

	// 顶层 panic recover 契约：panic 经 printError 输出 JSON envelope 并以
	// 退出码 2（printError 默认 HTTP 500 → ExitCode=2 服务端错误档）退出，
	// 同时 debug.Stack() 写 stderr 辅助定位。
	// recover 必须在 main 顶层 defer：cobra 内部不主动 recover Run 回调 panic。
	//
	// 退出码必须在此显式 os.Exit：panic 会中断 main 的顺序执行并直接进入
	// defer 链，函数尾部的 pendingExitCode 判断与 os.Exit 均不可达——若不在
	// 此退出，recover 之后 main 正常返回，进程以 exit 0 结束，脚本把 panic
	// 误判为成功（AST 静态测试只校验 recover/printError 字样存在，查不出
	// 控制流不可达；TestMain_PanicExitCode_IsTwo 以子进程实测锁定）。
	//
	// LIFO 顺序：本 defer 先于下方资源清理 defer 注册，故 panic 时它先执行；
	// 资源清理在 os.Exit 之前被跳过，因此这里显式调用 closeAllClients，
	// 与正常退出路径的清理保持一致。
	defer func() {
		if r := recover(); r != nil {
			printError(fmt.Errorf("内部错误: %v", r))
			// --quiet 契约：quiet 时不写 debug.Stack()（printError 自带 quiet 守卫）。
			// 非 quiet 时借用 recoverx.RecoverPanic 输出 stack 辅助定位。
			if !quiet {
				_ = recoverx.RecoverPanic(r, nil, "main")
			}
			if err := closeAllClients(); err != nil {
				printError(fmt.Errorf("关闭 Client 资源失败: %w", err))
			}
			if err := closeLogFiles(); err != nil {
				fmt.Fprintf(os.Stderr, "warn: 关闭日志文件失败: %v\n", err)
			}
			code := int(pendingExitCode.Load())
			if code == 0 {
				// printError 未设置退出码时的兜底：panic 绝不可被当作成功。
				code = 2
			}
			os.Exit(code)
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
	rootCmd.AddCommand(sessionCmd) // session parent（activate 子命令在 session.go init 内注册，勿在此重复添加——cobra AddCommand 不去重，双注册会让 --help 出现重复条目）

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
