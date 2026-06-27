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
			// 用 ErrorCategory 分类替代 errors.Is 逐一枚举。
			// 以下分类被视为 partial failure（有部分数据可用时输出 envelope）：
			//   - ErrorCategoryBusiness：业务错误（部分维度失败）
			//   - ErrorCategoryEmptyData：session 激活返回空用户数据
			//   - ErrorCategorySession：session 激活在冷却窗口被抑制
			// context 取消/超时独立于 ErrorCategory 系统（标准库错误，非 client 哨兵）。
			cat := client.ClassifyError(err)
			isPartialErr := cat == client.ErrorCategoryBusiness ||
				cat == client.ErrorCategoryEmptyData ||
				cat == client.ErrorCategorySession ||
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
	registerBizFlags(taskListCmd)
}
