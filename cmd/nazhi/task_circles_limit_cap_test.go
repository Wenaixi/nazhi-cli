package main

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

// CLI-123-01：task_teacher/task_public/task_submitted/task_withdrawn 四命令
// --limit 无上钳——SDK raw_json.go:409-411 遇 endPage>maxTotalPage 静默 endPage=1
// 只返首页快照，脚本拿截断数据不自知。rejectLoneOffset 已加 limit>maxCLILimit
// 参数错误拒绝（400/exit3），本测试锁定边界 100000 放行 / 100001 拒绝。
// 对齐 C-04 page_size_cap_test 范式（pageSizeListTestCmd 同款）。

// limitTaskCmd 构造带业务参数与指定 --limit 的任务命令（teacher 变体，四命令
// 共用 rejectLoneOffset 逻辑，仅验证单命令即覆盖全族）。
func limitTaskCmd(t *testing.T, baseURL, limit string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "task-limit"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Int("offset", 0, "")
	cmd.Flags().Int("limit", 0, "")
	_ = cmd.Flags().Set("limit", limit)
	cmd.Flags().Bool("count", false, "")
	cmd.Flags().String("key", "", "")
	return cmd
}

// TestTaskCircles_RejectHugeLimit 锁定 CLI-123-01：--limit 超 maxCLILimit(100000)
// 参数错误拒绝（exit3），边界 100000 放行（不设退出码）。
func TestTaskCircles_RejectHugeLimit(t *testing.T) {
	t.Run("limit 100001 拒绝", func(t *testing.T) {
		cmd := limitTaskCmd(t, "http://127.0.0.1:1", "100001")
		swapListGlobals(t)
		_, _, restore := captureStdio(t)
		taskTeacherCmd.Run(cmd, nil)
		restore()
		if pendingExitCode.Load() != 3 {
			t.Errorf("limit=100001 应走参数错误退出码 3，实际 %d", pendingExitCode.Load())
		}
	})

	t.Run("limit 100000 放行", func(t *testing.T) {
		cmd := limitTaskCmd(t, "http://127.0.0.1:1", "100000")
		swapListGlobals(t)
		_, _, restore := captureStdio(t)
		taskTeacherCmd.Run(cmd, nil)
		restore()
		// 放行后会走到 buildBizClient → 连接 127.0.0.1:1 失败 → 网络错误 exit2
		// （非参数错误 3），证明没被 limit 校验拦下。
		if pendingExitCode.Load() == 3 {
			t.Errorf("limit=100000 不应被参数校验拒绝，实际 exit3")
		}
	})
}
