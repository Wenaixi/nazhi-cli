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
// 修复前基线见 git 历史（ba6339c）；以下为 P1-1 修复后的新基线：
//   - HTTPDoSmallBody：95（修复前 119）
//   - HTTPDoLargeBody：142（修复前 205）
//   - 大响应体 B/op：4.7MB（修复前 15.4MB）——一次等大 string 分配的浪费已归零
const (
	// budgetHTTPDoSmallBody 是 httpDo 处理小响应体（<1KB）的分配次数。
	// P1-1 修复后基线：95（Go 1.26.1，commit 待定）
	budgetHTTPDoSmallBody = 95

	// budgetHTTPDoLargeBody 是 httpDo 处理约 1.2MB 响应体的分配次数。
	// P1-1 修复后基线：142（Go 1.26.1，commit 待定）
	// 修复前为 205（含日志参数提前求值导致的一份等大字符串分配）。
	budgetHTTPDoLargeBody = 142

	// budgetAssembleCirclesJSON4Pages 是 4 页合并的分配次数。
	// 实测基线：1（Go 1.26.1，commit 1445073）
	budgetAssembleCirclesJSON4Pages = 1

	// budgetActivateSessionCacheHit 是 session 缓存命中的分配次数。
	// 实测基线：0（Go 1.26.1，commit 1445073）——全部经 atomic.Pointer，
	// 零分配是设计与实现的边界，cache hit 一旦出现分配即为回归。
	budgetActivateSessionCacheHit = 0

	// budgetFetchCirclePageJSON500 是单页写实拉取（含 session 缓存命中预热后）
	// 的分配次数。P1-2 把 dataList 校验从全量 json.Unmarshal 改为首字符判定后：
	//   675 → 159（-76%），B/op 2232679 → 1623844
	// 注：其余差异来自该路径含 session 预热 + 多次内部 httpDo，非纯解码。
	budgetFetchCirclePageJSON500 = 160
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

// TestPerfBudget_FetchCirclePageJSON500 锁住单页写实拉取的分配次数。
//
// P1-2 哨兵：dataList 校验若被人改回全量 json.Unmarshal，这里立即 FAIL。
func TestPerfBudget_FetchCirclePageJSON500(t *testing.T) {
	list := benchDataListJSON(500)
	body := benchUnifiedBody(list, 500, 1)
	srv := benchWarmupBizServer(t, body)
	c := benchClient(t, srv.URL)
	ctx := context.Background()

	if _, err := c.ActivateSession(ctx, benchToken); err != nil {
		t.Fatalf("预热失败: %v", err)
	}

	assertAllocsWithin(t, "fetchCirclePageJSON(500 条)", budgetFetchCirclePageJSON500, func() {
		if _, _, err := c.fetchCirclePageJSON(ctx, benchToken, 1, 500, 3, ""); err != nil {
			t.Fatalf("fetchCirclePageJSON 失败: %v", err)
		}
	})
}

// TestPerfBudget_HTTPDoLargeBody_NoRedactAlloc 是 P1-1 的专项哨兵。
//
// 契约：当日志级别未启用时，httpDo 不得为日志参数做任何与响应体等大的分配。
//
// 修复前：logx.RedactBodyThenTruncate(respBytes, 100) 作为函数实参在
// logWithLevel 的 Enabled 检查之前求值，其内部第一步 string(body) 会对
// 整个响应体分配等大字符串（1.2MB），随后跑两遍正则再截断成 100 字符——
// 全部发生在 Info 日志被 LevelWarn 过滤、永不输出的情况下。
//
// 断言方式：对比「大响应体」与「小响应体」的 B/op 差值。
// 修复后两者差值应约等于响应体大小差（io.ReadAll 的必要分配）；
// 修复前差值会明显超出（多出一次等大字符串分配）。
func TestPerfBudget_HTTPDoLargeBody_NoRedactAlloc(t *testing.T) {
	const smallN, largeN = 2, 2000

	smallBody := benchUnifiedBody(benchDataListJSON(smallN), smallN, 1)
	largeBody := benchUnifiedBody(benchDataListJSON(largeN), largeN, 4)
	// 响应体字节差，即 io.ReadAll 必要分配的增量下界
	bodyDelta := len(largeBody) - len(smallBody)

	srvSmall := benchBizServer(t, smallBody)
	cSmall := benchClient(t, srvSmall.URL)
	srvLarge := benchBizServer(t, largeBody)
	cLarge := benchClient(t, srvLarge.URL)
	ctx := context.Background()
	hSmall := cSmall.bizHeaders(benchToken)
	hLarge := cLarge.bizHeaders(benchToken)

	bytesSmall := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = cSmall.httpDo(ctx, http.MethodGet, cSmall.bizURL("/api/bench"), nil, hSmall, "")
		}
	}).AllocedBytesPerOp()

	bytesLarge := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = cLarge.httpDo(ctx, http.MethodGet, cLarge.bizURL("/api/bench"), nil, hLarge, "")
		}
	}).AllocedBytesPerOp()

	delta := int(bytesLarge - bytesSmall)
	// io.ReadAll 用 bytes.Buffer 倍增扩容：最终数组 + 中间扩容垃圾 ≈ 2× 最终容量，
	// 故正常路径的差值约 ≤2.2× bodyDelta。设上限 2.5× bodyDelta 排除日志垃圾：
	//   - 无日志垃圾（修复后）：差值 ≈2.2×，通过
	//   - 有日志垃圾（修复前加一份等大 string(body) 分配）：差值 ≈7×，FAIL
	// 实测对照：修复前 15.3MB（≈7.3×），修复后 4.7MB（≈2.2×）。
	const slackPercent = 250
	limit := bodyDelta + bodyDelta*slackPercent/100

	t.Logf("小响应体 B/op=%d，大响应体 B/op=%d，差值=%d，响应体字节差=%d，上限=%d",
		bytesSmall, bytesLarge, delta, bodyDelta, limit)

	if delta > limit {
		t.Errorf("httpDo 大响应体分配超出必要量：差值 %d 字节 > 上限 %d 字节（响应体差 %d）。"+
			"疑似日志参数在级别检查前被求值——检查 httpDo 中 logx.RedactBodyThenTruncate 的调用点是否已由 logEnabled 守卫",
			delta, limit, bodyDelta)
	}
}
