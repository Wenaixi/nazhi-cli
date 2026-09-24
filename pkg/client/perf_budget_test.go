package client

import (
	"context"
	"net/http"
	"testing"
)

// ─── allocs/op 硬门禁 ───
//
// 为什么只卡 allocs 不卡时间：allocs 在同一 Go 版本下完全确定，不受 CI
// 机器负载影响；ns/op 在慢 runner 上会假阳性（CLAUDE.md 记录过 session
// warmup 本机 800ms / CI 4s+ 的教训，任何绝对时间断言都会炸）。
//
// 预算值 = 实测基线精确值，不留余量——留余量只会让小退步从门禁漏过。
// Go 版本升级导致数字变化时，属于有意识动作：更新常量并在下方注释记录原因。
//
// 运行：go test -count=1 -run TestPerfBudget ./pkg/client/ -v

// perfBudgetRuns 是 AllocsPerRun 的迭代次数。取值权衡：
//   - 太小：单次异常分配（如 GC 时机）会显著拉动平均值
//   - 太大：大响应体 fixture 会拖慢测试
//
// 100 次配合 ~64KB fixture，单测试耗时在毫秒级。
const perfBudgetRuns = 100

// 预算常量。每个常量的值为实测基线精确值（Go 1.26.1，本机 i9-12900HX）。
const (
	// budgetHTTPDoSmallBody 是 httpDo 处理小响应体（<1KB）的分配次数。
	// 实测基线：119（Go 1.26.1，commit 1445073）
	budgetHTTPDoSmallBody = 119

	// budgetHTTPDoLargeBody 是 httpDo 处理约 1.2MB 响应体的分配次数。
	// 实测基线：196（Go 1.26.1，commit 1445073）
	// 本值含 P1-1 的浪费（日志参数提前求值导致的等大字符串分配），
	// 修复后应显著下调。
	budgetHTTPDoLargeBody = 196

	// budgetAssembleCirclesJSON4Pages 是 4 页合并的分配次数。
	// 实测基线：1（Go 1.26.1，commit 1445073）
	budgetAssembleCirclesJSON4Pages = 1

	// budgetActivateSessionCacheHit 是 session 缓存命中的分配次数。
	// 实测基线：0（Go 1.26.1，commit 1445073）——全部经 atomic.Pointer，
	// 零分配是设计与实现的边界，cache hit 一旦出现分配即为回归。
	budgetActivateSessionCacheHit = 0
)

// assertAllocsWithin 是门禁断言 helper，统一错误消息格式。
func assertAllocsWithin(t *testing.T, name string, budget int, fn func()) {
	t.Helper()
	got := testing.AllocsPerRun(perfBudgetRuns, fn)
	if got > float64(budget) {
		t.Errorf("%s 分配次数超预算: got %.0f, budget %d（性能退步，检查最近的改动；确认是退步则回退该 commit）",
			name, got, budget)
	}
}

// TestPerfBudget_HTTPDoSmallBody 锁住小响应体的分配次数。
func TestPerfBudget_HTTPDoSmallBody(t *testing.T) {
	body := benchUnifiedBody(benchDataListJSON(2), 2, 1)
	srv := benchBizServer(t, body)
	c := benchClient(t, srv.URL)
	ctx := context.Background()
	headers := c.bizHeaders(benchToken)

	assertAllocsWithin(t, "httpDo(小响应体)", budgetHTTPDoSmallBody, func() {
		if _, err := c.httpDo(ctx, http.MethodGet, c.bizURL("/api/bench"), nil, headers, ""); err != nil {
			t.Fatalf("httpDo 失败: %v", err)
		}
	})
}

// TestPerfBudget_HTTPDoLargeBody 锁住大响应体的分配次数。
//
// 这是 P1-1 的哨兵：日志参数提前求值会让每次请求多分配一个与响应体等大的
// 字符串。修复后预算下调，任何让该分配复活的改动都会在此 FAIL。
func TestPerfBudget_HTTPDoLargeBody(t *testing.T) {
	body := benchUnifiedBody(benchDataListJSON(2000), 2000, 4)
	srv := benchBizServer(t, body)
	c := benchClient(t, srv.URL)
	ctx := context.Background()
	headers := c.bizHeaders(benchToken)

	assertAllocsWithin(t, "httpDo(大响应体)", budgetHTTPDoLargeBody, func() {
		if _, err := c.httpDo(ctx, http.MethodGet, c.bizURL("/api/bench"), nil, headers, ""); err != nil {
			t.Fatalf("httpDo 失败: %v", err)
		}
	})
}

// TestPerfBudget_AssembleCirclesJSON4Pages 锁住多页合并的分配次数。
func TestPerfBudget_AssembleCirclesJSON4Pages(t *testing.T) {
	raw1 := benchDataListJSON(500)
	results := make([]rawResult, 5)
	for pn := 2; pn <= 4; pn++ {
		results[pn] = rawResult{raw: raw1}
	}

	assertAllocsWithin(t, "assembleCirclesJSON(4 页)", budgetAssembleCirclesJSON4Pages, func() {
		if _, err := assembleCirclesJSON(raw1, results, 4, nil); err != nil {
			t.Fatalf("assembleCirclesJSON 失败: %v", err)
		}
	})
}

// TestPerfBudget_ActivateSessionCacheHit 锁住 session 缓存命中路径的分配次数。
//
// 长驻 SDK 场景下每次业务调用都走这里，分配数直接乘调用次数。
func TestPerfBudget_ActivateSessionCacheHit(t *testing.T) {
	srv := benchWarmupBizServer(t, []byte(`{"code":1,"msg":"成功"}`))
	c := benchClient(t, srv.URL)
	ctx := context.Background()

	if _, err := c.ActivateSession(ctx, benchToken); err != nil {
		t.Fatalf("预热失败: %v", err)
	}

	assertAllocsWithin(t, "ActivateSession(缓存命中)", budgetActivateSessionCacheHit, func() {
		if _, err := c.ActivateSession(ctx, benchToken); err != nil {
			t.Fatalf("ActivateSession 失败: %v", err)
		}
	})
}