package main

import (
	"encoding/json"
	"fmt"

	"github.com/Wenaixi/nazhi-cli/pkg/envelope"
	"github.com/spf13/cobra"
)

// taskSubmittedCmd 表示 nazhi task submitted 命令
//
//	nazhi task submitted --token <token> [--base-url <url>] [--timeout <秒>]
//
// 同时作为 `task done` 别名注册（语义更直白）。
//
// envelope.data 直接透传 SDK GetSubmittedCirclesJSON 返回的原始 JSON 数组
// （跨页合并后），与平台响应 byte-for-byte 一致。
var taskSubmittedCmd = &cobra.Command{
	Use:   "submitted",
	Short: "获取已提交写实记录",
	Long: `调用 getStudentCircle 接口，获取当前用户已提交的全部写实记录（含正文、图片、审核状态）。
自动翻页合并，输出全量数据。

envelope.data 是平台原始 JSON 数组，total 可用 jq '.data | length' 取。`,
	Example: `  nazhi task submitted --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi task done --token eyJhbGciOiJIUzI1NiJ9.xxx      # 同 submitted，别名`,
	Run: func(cmd *cobra.Command, args []string) {
		c, token, err := buildBizClient(cmd)
		if err != nil {
			printError(err)
			return
		}

		printVerbose("正在获取已提交写实记录...")
		raw, err := c.GetSubmittedCirclesJSON(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("获取已提交写实记录失败: %w", err))
			return
		}
		if len(raw) == 0 {
			printEnvelope(envelope.Success(json.RawMessage("[]")))
			return
		}

		printEnvelope(envelope.Success(json.RawMessage(raw)))
	},
}

// taskDoneCmd 是 task submitted 的别名（语义更直白）。
// 共用 taskSubmittedCmd.Run 回调，避免逻辑重复。
var taskDoneCmd = &cobra.Command{
	Use:   "done",
	Short: "查看已提交的写实记录 (task submitted 别名)",
	Run:   taskSubmittedCmd.Run,
}

func init() {
	registerBizFlags(taskSubmittedCmd)
	registerBizFlags(taskDoneCmd)
}
