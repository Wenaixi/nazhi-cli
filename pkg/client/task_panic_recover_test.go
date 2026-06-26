// Package client 内部白盒测试 — 验证 G2 修复。
//
// G2 (round-7) 修复：errgroup.Go 闭包内 fetchTasksForDimension 无 panic recover。
// 任何 panic（nil deref / 第三方库 bug）会逃逸到 runtime → 进程崩溃 → g.Wait() 永不返回。
//
// 修复：fetchTasksForDimensionSafe 包裹 defer recover，panic 转成 error 进 dimErrs。
//
// 本测试用 package client（同包）访问私有方法。
package client

import (
	"context"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestFetchTasksForDimensionSafe_RecoversFromPanic 白盒验证 G2 修复。
//
// 构造路径：c.http=nil → fetchTasksForDimension 内部 c.activateSessionIfNeeded
// 调用 c.http.Do() → nil pointer dereference panic。
// fetchTasksForDimensionSafe 的 defer recover 把 panic 转成 error 返回。
//
// 旧行为：g.Go 闭包内 panic 逃逸 → 进程崩溃 → g.Wait 永不返回 → 测试进程崩溃。
// 新行为：panic 被捕获 → 返回 (nil, error) → 调用方拿到 error 进 dimErrs。
func TestFetchTasksForDimensionSafe_RecoversFromPanic(t *testing.T) {
	// 构造 *Client：ssoBaseURL/baseURL 有效但 http=nil（零值）
	// 触发 fetchTasksForDimension 内部 c.activateSessionIfNeeded → c.http.Do() → nil deref panic
	c := &Client{
		ssoBaseURL: "http://example.com",
		baseURL:    "http://example.com",
	}
	dim := types.Dimension{ID: 1, Name: "panic-dim"}
	headers := map[string]string{"X-Auth-Token": "test-token"}

	// 关键断言 1：调用不 panic 到 runtime（说明 recover 起作用）
	// 关键断言 2：返回 (nil, error)，error 信息含 "panic"
	tasks, err := c.fetchTasksForDimensionSafe(context.Background(), dim, headers)
	if tasks != nil {
		t.Errorf("期望 tasks=nil，实际 %v", tasks)
	}
	if err == nil {
		t.Fatal("期望 panic 转成 error 返回，实际 nil")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("err 应含 panic 信息，实际: %v", err)
	}
	if !strings.Contains(err.Error(), "panic-dim") {
		t.Errorf("err 应含 dim 名称 panic-dim 便于定位，实际: %v", err)
	}
	t.Logf("G2 修复验证：panic 被 recover 捕获，err=%v", err)
}
