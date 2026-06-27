package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// taskListCmd 表示 nazhi task list 命令
//
//	nazhi task list --token <token> [--base-url <url>] [--timeout <秒>]
var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取全维度任务列表",
	Long:  `拉取目标平台全部维度的任务列表。内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。`,
	Example: `  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx
	  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取任务列表...")
		tasks, err := c.FetchTasks(cmd.Context(), token)
		if err != nil {
			// 区分「全失败」与「部分失败」。
			// SDK 设计契约：FetchTasks 在「单维度失败但其他维度成功」时
			// 返回 (tasks, ErrBusinessRejected)。此时下游仍想拿到成功的任务数据
			// 但也需要被告知存在 partial failure。
			// 旧实现：err != nil 一律走 printError → return，stdout 空，下游拿不到任何任务。
			// 新实现：partial failure 走 envelope（stdout 输出 status+tasks+error）
			//         其他错误（全失败/网络失败）维持原 printError 路径。
			//         envelope 场景下 pendingExitCode 标记为 1，CI 脚本可通过 exit code 区分。
			// failed_count 不在 envelope 中：调用方解析 err.Error() 字符串中的
			// "维度 X 业务错误" 段即可得到（FetchTasks 会把每个失败维度的 ID/name/msg
			// 拼接到错误消息里）。此处不再做二次解析避免重复实现。
			// 扩展 envelope 分支覆盖 ErrEmptyUserInfo。
			// F9 只覆盖 ErrBusinessRejected，但当 session 预热步骤 4
			// getMyInfo 返回空数据时 FetchTasks 会返回 (nil, ErrEmptyUserInfo)
			// 旧 envelope 分支不匹配 → 走 printError → stdout 空，下游拿不到
			// 成功维度的数据（即使 len(tasks) > 0 也因 ErrEmptyUserInfo 走了
			// printError 路径）。
			// 修复：|| errors.Is(err, client.ErrEmptyUserInfo) 让两种业务错误
			// 都走 envelope 路径。
			// 注意：ErrEmptyUserInfo 通常发生在 session 激活阶段（此时 tasks
			// 为 nil），len(tasks) > 0 条件在正常情况下会拦截。留下这个分支
			// 作为防御性编程——避免未来代码演化后 ErrEmptyUserInfo 出现在
			// 任务已部分获取的路径。
			// 扩展 envelope 分支覆盖 context 取消与 ErrSessionBackoff。
			// 旧实现只覆盖 ErrBusinessRejected + ErrEmptyUserInfo
			// 当 FetchTasks 因 ctx cancel 或 session backoff 路径返回错误时
			// envelope 分支全 false → 走 printError 丢 partial tasks。
			// 修复后覆盖以下语义，调用方都能拿到 partial tasks
			//   - ErrBusinessRejected：业务错误（部分维度失败）
			//   - ErrEmptyUserInfo：session 激活返回空用户数据
			//   - context.Canceled / DeadlineExceeded：context 取消/超时
			//   - ErrSessionBackoff：session 激活在冷却窗口被抑制
			isPartialErr := errors.Is(err, client.ErrBusinessRejected) ||
				errors.Is(err, client.ErrEmptyUserInfo) ||
				errors.Is(err, client.ErrSessionBackoff) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded)
			if isPartialErr && len(tasks) > 0 {
				printJSON(map[string]any{
					"status": "partial",
					"reason": "fetch_tasks_partial_failure",
					"tasks":  tasks,
					"error":  err.Error(),
				})
				markError() // 与 F7 模式一致：标记退出码为 1 但不调用 os.Exit
				return
			}
			printError(fmt.Errorf("获取任务列表失败: %w", err))
			return
		}

		printJSON(tasks)
	},
}

func init() {
	taskListCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	taskListCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	taskListCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
