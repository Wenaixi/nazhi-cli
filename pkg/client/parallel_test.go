package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── 空维度/跳过 ID=0 ───

func TestParallelDims_EmptyDims(t *testing.T) {
	result, egErr := ParallelDims[int](context.Background(), nil, 0,
		func(_ context.Context, _ types.Dimension) ([]int, error) {
			return nil, nil
		})
	if egErr != nil {
		t.Errorf("egErr 应为 nil, 得到 %v", egErr)
	}
	if len(result.Items) != 0 {
		t.Error("空维度应返回空 items")
	}
}

func TestParallelDims_SkipDimIDZero(t *testing.T) {
	dims := []types.Dimension{
		{ID: 0, Name: "全部"},
		{ID: 1, Name: "dim1"},
	}
	result, egErr := ParallelDims(context.Background(), dims, 0,
		func(_ context.Context, dim types.Dimension) ([]int, error) {
			return []int{int(dim.ID)}, nil
		})
	if egErr != nil {
		t.Errorf("egErr 应为 nil, 得到 %v", egErr)
	}
	if len(result.Items) != 1 || result.Items[0] != 1 {
		t.Errorf("应跳过 dim ID=0, items=%v", result.Items)
	}
}

// ─── 全部成功 ───

func TestParallelDims_AllSucceed(t *testing.T) {
	dims := []types.Dimension{
		{ID: 1, Name: "a"},
		{ID: 2, Name: "b"},
		{ID: 3, Name: "c"},
	}
	result, egErr := ParallelDims(context.Background(), dims, 2,
		func(_ context.Context, dim types.Dimension) ([]int, error) {
			return []int{int(dim.ID)}, nil
		})
	if egErr != nil {
		t.Fatalf("egErr 应为 nil, 得到 %v", egErr)
	}
	if len(result.Items) != 3 {
		t.Errorf("期望 3 items, 得到 %d", len(result.Items))
	}
	if result.CancelledCount != 0 || result.FailedCount != 0 {
		t.Errorf("不应有错误, Cancelled=%d Failed=%d", result.CancelledCount, result.FailedCount)
	}
}

// ─── 部分维度的业务错误 ───

func TestParallelDims_PartialBizError(t *testing.T) {
	dims := []types.Dimension{
		{ID: 1, Name: "ok"},
		{ID: 2, Name: "fail"},
		{ID: 3, Name: "ok2"},
	}
	result, egErr := ParallelDims(context.Background(), dims, 2,
		func(_ context.Context, dim types.Dimension) ([]string, error) {
			if dim.ID == 2 {
				return nil, errors.New("dim 2 biz error")
			}
			return []string{dim.Name}, nil
		})

	if egErr != nil {
		t.Fatalf("egErr 应为 nil, 得到 %v", egErr)
	}
	if len(result.Items) != 2 {
		t.Errorf("期望 2 个成功 item, 得到 %d", len(result.Items))
	}
	if result.FailedCount != 1 {
		t.Errorf("FailedCount 应为 1, 得到 %d", result.FailedCount)
	}
	if result.CancelledCount != 0 {
		t.Errorf("CancelledCount 应为 0, 得到 %d", result.CancelledCount)
	}
	if len(result.BizErrors) != 1 {
		t.Errorf("期望 1 个 biz error, 得到 %d", len(result.BizErrors))
	}
}

// ─── 全部维度业务失败 ───

func TestParallelDims_AllBizErrors(t *testing.T) {
	dims := []types.Dimension{
		{ID: 1, Name: "a"},
		{ID: 2, Name: "b"},
	}
	result, egErr := ParallelDims(context.Background(), dims, 0,
		func(_ context.Context, _ types.Dimension) ([]string, error) {
			return nil, errors.New("always fail")
		})
	if egErr != nil {
		t.Fatalf("egErr 应为 nil, 得到 %v", egErr)
	}
	if len(result.Items) != 0 {
		t.Error("全失败时应返回空 item 列表")
	}
	if result.FailedCount != 2 {
		t.Errorf("FailedCount 应为 2, 得到 %d", result.FailedCount)
	}
}

// ─── context 取消 ───

func TestParallelDims_ContextCancel(t *testing.T) {
	dims := []types.Dimension{
		{ID: 1, Name: "slow1"},
		{ID: 2, Name: "slow2"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	// 等待 cancel 生效后开始并行请求
	<-ctx.Done()

	result, _ := ParallelDims(ctx, dims, 1,
		func(_ context.Context, dim types.Dimension) ([]int, error) {
			return []int{int(dim.ID)}, nil
		})

	// 上下文已取消，可能全部 goroutine 都检测到 cancel 返回 non-nil 给 errgroup
	// 也可能某些 goroutine 在 cancel 前已启动成功。不强制 assert 具体行为，
	// 只验证返回不 panic + 错误分类合理（context error 应进入 CancelledCount 而不是 BizErrors）
	if result.CancelledCount == 0 && result.FailedCount > 0 {
		t.Errorf("context 取消下不应产生 biz 错误")
	}
}
