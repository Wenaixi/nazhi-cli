package main

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// CLI-124-09/10：四任务命令 --count 分支此前在 rejectLoneOffset 之前 return——
// --count --offset 5 / --count --limit -1 绕过校验静默返回 total，分页脚本拿错
// 形状不自知；--count --limit 5 语义互斥但静默忽略。本测试锁定 count 与
// offset/limit 冲突组合必须以参数错误（400/exit3）拒绝，且不发任何网络请求。

// countConflictCmd 构造带 --count 与冲突 offset/limit 的任务命令。
func countConflictCmd(t *testing.T, baseURL, offset, limit string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "task-count"}
	cmd.SetContext(context.Background())
	cmd.Flags().String("token", "", "")
	_ = cmd.Flags().Set("token", "test-token")
	cmd.Flags().String("base-url", "", "")
	_ = cmd.Flags().Set("base-url", baseURL)
	cmd.Flags().Int("timeout", 5, "")
	cmd.Flags().Bool("count", false, "")
	_ = cmd.Flags().Set("count", "true")
	cmd.Flags().Int("offset", 0, "")
	_ = cmd.Flags().Set("offset", offset)
	cmd.Flags().Int("limit", 0, "")
	_ = cmd.Flags().Set("limit", limit)
	cmd.Flags().String("key", "", "")
	return cmd
}

// TestTaskCircles_CountConflictsRejected 锁定 CLI-124-09/10：
// --count --offset 5（单独 offset）→ 400/exit3；--count --offset -1 → 400/exit3；
// --count --limit 5 → 400/exit3（语义互斥）。修复前 count 分支在 rejectLoneOffset
// 前 return 全部静默通过。
func TestTaskCircles_CountConflictsRejected(t *testing.T) {
	cases := []struct {
		name   string
		offset string
		limit  string
	}{
		{"count+offset 5", "5", "0"},
		{"count+offset -1", "-1", "0"},
		{"count+limit 5", "0", "5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 地址指向不可达端口：若校验被绕过会尝试网络请求 → 网络错误而非参数错误
			cmd := countConflictCmd(t, "http://127.0.0.1:1", tc.offset, tc.limit)
			swapListGlobals(t)
			_, _, restore := captureStdio(t)
			taskTeacherCmd.Run(cmd, nil)
			restore()
			if got := pendingExitCode.Load(); got != 3 {
				t.Errorf("--count+%s/%s 应走参数错误退出码 3，实际 %d", tc.offset, tc.limit, got)
			}
		})
	}
}

// TestTaskCircles_CountWithoutConflict_IssuesRequest 锁定 CLI-124-09 反面：
// 纯 --count（无 offset/limit）应正常放行并发出总数请求（不被误拒）。
func TestTaskCircles_CountWithoutConflict_IssuesRequest(t *testing.T) {
	srv, bizHit := pageSizeMockServer(t, "/api/studentCircleNew/getStudentCircle")
	cmd := countConflictCmd(t, srv.URL, "0", "0")
	swapListGlobals(t)
	stdout, _, restore := captureStdio(t)
	taskSubmittedCmd.Run(cmd, nil)
	restore()
	if !*bizHit {
		t.Fatalf("纯 --count 应放行并发出业务请求；stdout=%q", stdout.String())
	}
	if got := pendingExitCode.Load(); got != 0 {
		t.Errorf("纯 --count 不应设置退出码，实际 %d", got)
	}
}

var _ = strings.Contains // 保持 strings import 一致性（上方捕获输出用法）
