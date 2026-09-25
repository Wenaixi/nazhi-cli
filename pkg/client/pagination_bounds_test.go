package client

import "testing"

// TestDerivePageBounds_ 页数下界推导契约。
//
// 背景（v1.6.4 / 84982af）：服务端 totalPage 来自单一字段声明，可能虚低或为 0。
// 若只信 totalPage，翻页会静默漏数据。因此抓取页数必须取
// max(totalPage, ceil(totalNum/pageSize))。
//
// 本测试是提取纯函数 derivePageBounds 的 RED 阶段：函数尚不存在。
func TestDerivePageBounds_UsesTotalNumAsLowerBound(t *testing.T) {
	cases := []struct {
		name      string
		totalNum  int
		totalPage int
		pageSize  int
		want      int
	}{
		// totalPage 虚低：以 totalNum 推导为准
		{"totalPage 为 0 时用 totalNum 推导", 250, 0, 100, 3},
		{"totalPage 为 1 但 totalNum 覆盖 3 页", 250, 1, 100, 3},
		{"totalPage 虚低一半", 200, 2, 100, 2},
		// totalPage 虚高：规则是取 max，故采纳服务端声明值。
		// 实际流程中 totalNum<=pageSize 会在调用方提前短路，不会走到这里。
		{"totalPage 远大于 totalNum 时取声明值", 50, 99, 100, 99},
		// 整除边界
		{"totalNum 恰好整除", 200, 2, 100, 2},
		{"totalNum 差一条即多一页", 201, 2, 100, 3},
		// 正常场景
		{"单页覆盖", 50, 1, 500, 1},
		// 页数下界恒为 1：results 索引槽以页号下标存放，首页固定在 [1]
		{"totalNum 为 0 时仍为 1", 0, 0, 500, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := derivePageBounds(tc.totalNum, tc.totalPage, tc.pageSize)
			if got != tc.want {
				t.Errorf("derivePageBounds(totalNum=%d, totalPage=%d, pageSize=%d) = %d, 期望 %d",
					tc.totalNum, tc.totalPage, tc.pageSize, got, tc.want)
			}
		})
	}
}

// TestDerivePageBounds_ClampedToMaxTotalPage 锁定资源闸：页数上界钳制。
// maxTotalPage 之外的声明值不得直接驱动 make 分配（OOM 防护）。
func TestDerivePageBounds_ClampedToMaxTotalPage(t *testing.T) {
	got := derivePageBounds(1_000_000, 1_000_000, 100)
	if got != maxTotalPage {
		t.Errorf("页数应被钳制到 maxTotalPage(%d)，实际 %d", maxTotalPage, got)
	}
}

// TestClampPage_NoOpWhenWithinBound 锁定钳制辅助：未超界时原样返回。
func TestClampPage_NoOpWhenWithinBound(t *testing.T) {
	if got := clampPage(5); got != 5 {
		t.Errorf("未超界时应原样返回，实际 %d", got)
	}
}

// TestClampPage_MinOne 锁定下界：页数至少为 1（否则 results 索引槽无首页位）。
func TestClampPage_MinOne(t *testing.T) {
	if got := clampPage(0); got != 1 {
		t.Errorf("页数 0 应被抬升到 1，实际 %d", got)
	}
	if got := clampPage(-5); got != 1 {
		t.Errorf("负页数应被抬升到 1，实际 %d", got)
	}
}

// TestLimitEndPage_ConvergesToNeededPages 锁定 limit 路径的 endPage 推导：
// endPage = min(ceil((offset+limit)/pageSize), 声明/推导页数)。
func TestLimitEndPage_ConvergesToNeededPages(t *testing.T) {
	cases := []struct {
		name      string
		offset    int
		limit     int
		totalNum  int
		totalPage int
		pageSize  int
		want      int
	}{
		{"单页内", 0, 10, 100, 1, 100, 1},
		{"offset 跨页", 3, 2, 100, 5, 2, 3},
		{"limit 覆盖多页", 0, 250, 1000, 10, 100, 3},
		{"endPage 受 totalNum 收敛", 0, 10000, 50, 1, 100, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			declared := derivePageBounds(tc.totalNum, tc.totalPage, tc.pageSize)
			got := limitEndPage(tc.offset, tc.limit, tc.pageSize, declared)
			if got != tc.want {
				t.Errorf("limitEndPage(offset=%d, limit=%d, pageSize=%d, declared=%d) = %d, 期望 %d",
					tc.offset, tc.limit, tc.pageSize, declared, got, tc.want)
			}
		})
	}
}
