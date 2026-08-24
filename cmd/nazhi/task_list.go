package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskListCmd 表示 nazhi task list 命令
//
//	nazhi task list --token <token> [--base-url <url>] [--timeout <秒>]
//
// CLI 透传 SDK FetchTasks 的最终业务模型输出。
// SDK 在进入 CLI 前就已完成字段语义整理：
//   - circleTaskStatus → submitted
//   - upPic → needPic
//   - 日期字段为 string 透传（服务端原始格式，如 "2026-01-12"）
var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取全维度任务列表",
	Long:  `拉取目标平台全部维度的任务列表。内部流程：ActivateSession → getDimensions → 遍历维度 getCircleStatistics → 聚合。`,
	Example: `  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printParamError(err)
			return
		}

		printVerbose("正在获取任务列表...")
		tasks, err := c.FetchTasks(cmd.Context(), token)
		if err != nil {
			isPartialErr := errors.Is(err, client.ErrBusinessRejected) ||
				errors.Is(err, client.ErrEmptyUserInfo) ||
				errors.Is(err, client.ErrSessionBackoff) ||
				errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded)
			if isPartialErr && len(tasks) > 0 {
				printEnvelope(envelope.Partial(207, "fetch_tasks_partial_failure: "+err.Error(), tasks))
				return
			}
			printError(fmt.Errorf("获取任务列表失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(tasks))
	},
}

func init() {
	registerBizFlags(taskListCmd)
}
