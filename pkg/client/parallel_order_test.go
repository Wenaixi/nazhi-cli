package client

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestParallelDims_PreservesDimensionOrder 锁定聚合结果的维度声明顺序：
// 无论 goroutine 完成顺序如何（此处让 dim2 先完成），Items 必须按 dims 的
// 声明顺序拼接（dim1 的任务在前）。CLI task list 输出确定性依赖此行为。
func TestParallelDims_PreservesDimensionOrder(t *testing.T) {
	dims := []types.Dimension{
		{ID: 1, Name: "思想品德"},
		{ID: 2, Name: "学业水平"},
	}

	var slowFirst atomic.Bool
	fn := func(ctx context.Context, dim types.Dimension) ([]types.Task, error) {
		if dim.ID == 1 {
			// 维度1 故意放慢：若实现按完成序追加，dim2 会先入 Items 导致顺序翻转
			time.Sleep(50 * time.Millisecond)
			slowFirst.Store(true)
			return []types.Task{{ID: 11, Name: "d1-任务"}}, nil
		}
		return []types.Task{
			{ID: 21, Name: "d2-任务甲"},
			{ID: 22, Name: "d2-任务乙"},
		}, nil
	}

	result, err := ParallelDims[types.Task](context.Background(), dims, 8, fn)
	if err != nil {
		t.Fatalf("ParallelDims 失败: %v", err)
	}
	if !slowFirst.Load() {
		t.Fatal("测试前提未成立：维度1 未晚于维度2 完成，夹具失效")
	}
	if result.FailedCount != 0 || result.CancelledCount != 0 {
		t.Fatalf("不应有失败: biz=%v ctx=%v", result.BizErrors, result.ContextErrors)
	}
	if len(result.Items) != 3 {
		t.Fatalf("期望 3 条任务，得到 %d", len(result.Items))
	}
	wantOrder := []int64{11, 21, 22}
	for i, want := range wantOrder {
		if result.Items[i].ID != want {
			t.Fatalf("顺序错乱：位置 %d 期望 ID=%d，实际 ID=%d（完整顺序=%v）",
				i, want, result.Items[i].ID, taskIDs(result.Items))
		}
	}
}

func taskIDs(tasks []types.Task) []int64 {
	ids := make([]int64, len(tasks))
	for i, tk := range tasks {
		ids[i] = tk.ID
	}
	return ids
}