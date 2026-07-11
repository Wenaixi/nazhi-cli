package main

import (
	"context"
	"encoding/json"
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
// envelope.data 直接透传 SDK FetchTasksJSON 的跨维度合并原始 JSON 数组，
// 保留 platform 原始字段（如 circleTaskStatus / upPic 等），与平台响应 1:1。
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
		raw, err := c.FetchTasksJSON(cmd.Context(), token)
		if err != nil {
			// partial 语义：raw 非 nil 表示已有部分维度数据可用，按 partial envelope 输出。
			// raw 为 nil 表示全部失败，走 printError。
			if len(raw) > 0 {
				isPartial := errors.Is(err, client.ErrBusinessRejected) ||
					errors.Is(err, client.ErrEmptyUserInfo) ||
					errors.Is(err, client.ErrSessionBackoff) ||
					errors.Is(err, context.Canceled) ||
					errors.Is(err, context.DeadlineExceeded)
				if isPartial {
					printEnvelope(envelope.Partial(207, "fetch_tasks_partial_failure: "+err.Error(),
						json.RawMessage(raw)))
					return
				}
			}
			printError(fmt.Errorf("获取任务列表失败: %w", err))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

func init() {
	registerBizFlags(taskListCmd)
}
