package client

import (
	"testing"
)

// TestBudgetTruncatePage 锁定预算截断纯函数语义（候选 7 收敛后单点）。
// 此前该回退循环在 getCirclesJSON / getCirclesLimitJSON 中逐字重复两份，
// 只能经全链路 HTTP mock 间接触达；抽为纯函数后可脱离组装直测。
func TestBudgetTruncatePage(t *testing.T) {
	// raw1 是首页原始字节；results 的 raw 按页模拟累积量。
	raw1 := []byte("abcdefgh") // 8 字节

	t.Run("未命中预算返回原页号+false", func(t *testing.T) {
		results := make([]rawResult, 3) // 页 1..2 各 4 字节，累积 16 < 预算
		results[2] = rawResult{raw: []byte("1234")}
		page, hit := budgetTruncatePage(raw1, results, 2)
		if hit {
			t.Fatalf("不应命中预算: got hit=true")
		}
		if page != 2 {
			t.Fatalf("未命中应返回声明页号: got %d want 2", page)
		}
	})

	t.Run("命中预算回退到合法前缀", func(t *testing.T) {
		// 构造超预算：页 2 raw 极大（> maxAssembleBuffer），页 1 累积 = 8 + 极大 > 预算
		results := make([]rawResult, 4)
		huge := make([]byte, int(maxAssembleBuffer)+1024)
		results[3] = rawResult{raw: huge} // 页 3 超预算
		page, hit := budgetTruncatePage(raw1, results, 3)
		if !hit {
			t.Fatalf("超预算应命中: got hit=false")
		}
		// 回退到页 2：累积 = 8 + 页2(空=0) = 8 ≤ 预算
		if page != 2 {
			t.Fatalf("应回退到页 2: got %d", page)
		}
	})

	t.Run("untilPage<1 时按未命中处理", func(t *testing.T) {
		results := make([]rawResult, 1)
		page, hit := budgetTruncatePage(raw1, results, 0)
		if hit {
			t.Fatalf("untilPage=0 不应命中: got hit=true")
		}
		if page != 0 {
			t.Fatalf("应返回声明页号 0: got %d", page)
		}
	})

	t.Run("预算内逐页回退到底", func(t *testing.T) {
		// 页 2..3 均超过预算 → 回退到页 1（仅首页 8 字节）
		results := make([]rawResult, 4)
		huge := make([]byte, int(maxAssembleBuffer)+1024)
		results[2] = rawResult{raw: huge}
		results[3] = rawResult{raw: huge}
		page, hit := budgetTruncatePage(raw1, results, 3)
		if !hit {
			t.Fatalf("超预算应命中: got hit=false")
		}
		if page != 1 {
			t.Fatalf("应回退到仅首页页 1: got %d", page)
		}
	})
}
