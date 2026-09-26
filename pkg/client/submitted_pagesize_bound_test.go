package client

import (
	"io"
	"log/slog"
	"math"
	"testing"
)

// newPageSizeBoundClient 构造只用于 Option 守卫测试的最小 Client。
// Option 只读写 logger 与 submittedPageSize，无需真实 HTTP 端点。
func newPageSizeBoundClient() *Client {
	return &Client{
		logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		submittedPageSize: defaultSubmittedPageSize,
	}
}

// TestMaxSubmittedCapacityCeiling_NoOverflow 锁定容量上限计算不产生整数溢出。
//
// 背景：翻页路径曾用 `maxTotalPage * pageSize` 作为容量钳制上界。pageSize
// 极大时该乘法在 int 上回绕为负，负值与 capacity 比较恒为假，钳制失效，
// 后续 make 拿到负容量直接 panic。本测试覆盖安全与溢出两条分支。
func TestMaxSubmittedCapacityCeiling_NoOverflow(t *testing.T) {
	cases := []struct {
		name     string
		pageSize int
		want     int
	}{
		{"常规页长 500", 500, maxTotalPage * 500},
		{"1", 1, maxTotalPage},
		{"Option 上界 1<<20", 1 << 20, maxTotalPage * (1 << 20)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := maxSubmittedCapacityCeiling(c.pageSize)
			if got != c.want {
				t.Fatalf("期望 %d，实际 %d", c.want, got)
			}
			if got < 0 {
				t.Fatalf("容量上限不应为负，实际 %d", got)
			}
		})
	}

	// 溢出分支：pageSize 超过 MaxInt/maxTotalPage 时应饱和到 math.MaxInt，
	// 绝不能回绕为负或中间值。
	overflowSize := math.MaxInt/maxTotalPage + 1
	got := maxSubmittedCapacityCeiling(overflowSize)
	if got != math.MaxInt {
		t.Fatalf("溢出时应饱和到 math.MaxInt，实际 %d", got)
	}
	if got <= 0 {
		t.Fatalf("饱和值必须为正，实际 %d", got)
	}
}

// TestWithSubmittedPageSize_RejectsAboveBound 锁定 Option 层上界：
// 超过上界的值被拒绝并保持当前值，避免病态 pageSize 流入下游计算。
func TestWithSubmittedPageSize_RejectsAboveBound(t *testing.T) {
	t.Run("超上界被拒绝", func(t *testing.T) {
		c := newPageSizeBoundClient()
		before := c.submittedPageSize
		WithSubmittedPageSize(maxSubmittedPageSize + 1)(c)
		if c.submittedPageSize != before {
			t.Fatalf("超上界值应被拒绝，pageSize 从 %d 变为 %d", before, c.submittedPageSize)
		}
	})

	t.Run("非正数被拒绝", func(t *testing.T) {
		c := newPageSizeBoundClient()
		before := c.submittedPageSize
		for _, n := range []int{0, -1, -500} {
			WithSubmittedPageSize(n)(c)
			if c.submittedPageSize != before {
				t.Fatalf("非正数 %d 应被拒绝，实际 pageSize=%d", n, c.submittedPageSize)
			}
		}
	})

	t.Run("合法值被接受", func(t *testing.T) {
		c := newPageSizeBoundClient()
		for _, n := range []int{1, 100, 500, maxSubmittedPageSize} {
			WithSubmittedPageSize(n)(c)
			if c.submittedPageSize != n {
				t.Fatalf("合法值 %d 应被接受，实际 %d", n, c.submittedPageSize)
			}
		}
	})
}
