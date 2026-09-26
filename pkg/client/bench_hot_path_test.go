package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── 热点路径 benchmark ───
//
// 只产出数字，不做断言——门禁在 perf_budget_test.go。
// 时间指标（ns/op）在 CI 慢 runner 上会假阳性（CLAUDE.md 有记录：
// session warmup 本机 800ms、CI 4s+），故 ns/op 只作本地对比参考。
// 真正防退步的是 allocs/op 硬门禁。
//
// 运行：
//   go test -run '^$' -bench . -benchmem ./pkg/client/

const benchToken = "bench-token-not-a-real-credential"

// BenchmarkHTTPDo_SmallBody 测小响应体（典型业务响应 <1KB）的处理开销。
// 这是「session 缓存命中后的业务请求」场景的基准。
func BenchmarkHTTPDo_SmallBody(b *testing.B) {
	body := benchUnifiedBody(benchDataListJSON(2), 2, 1)
	srv := benchBizServer(b, body)
	c := benchClient(b, srv.URL)
	ctx := context.Background()
	headers := c.bizHeaders(benchToken)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.httpDo(ctx, http.MethodGet, c.bizURL("/api/bench"), nil, headers, ""); err != nil {
			b.Fatalf("httpDo 失败: %v", err)
		}
	}
}

// BenchmarkHTTPDo_LargeBody 测大响应体（约 1.2MB，对齐真实公示页量级）。
// 的核心观测点：日志参数提前求值会让每次请求多分配一个与响应体等大的字符串。
func BenchmarkHTTPDo_LargeBody(b *testing.B) {
	body := benchUnifiedBody(benchDataListJSON(2000), 2000, 4)
	srv := benchBizServer(b, body)
	c := benchClient(b, srv.URL)
	ctx := context.Background()
	headers := c.bizHeaders(benchToken)

	b.SetBytes(int64(len(body)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.httpDo(ctx, http.MethodGet, c.bizURL("/api/bench"), nil, headers, ""); err != nil {
			b.Fatalf("httpDo 失败: %v", err)
		}
	}
}

// BenchmarkFetchCirclePageJSON_500 测单页写实拉取全链路（含 session 预热缓存命中）。
func BenchmarkFetchCirclePageJSON_500(b *testing.B) {
	list := benchDataListJSON(500)
	body := benchUnifiedBody(list, 500, 1)
	srv := benchWarmupBizServer(b, body)
	c := benchClient(b, srv.URL)
	ctx := context.Background()

	// 预热一次让 sm 缓存生效，避免把 4 步激活算进每次迭代
	if _, err := c.ActivateSession(ctx, benchToken); err != nil {
		b.Fatalf("预热失败: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := c.fetchCirclePageJSON(ctx, benchToken, 1, 500, 3, ""); err != nil {
			b.Fatalf("fetchCirclePageJSON 失败: %v", err)
		}
	}
}

// BenchmarkAssembleCirclesJSON_4Pages 测多页合并（纯内存，无网络）。
// 这是大响应体场景的主要内存峰值点。
func BenchmarkAssembleCirclesJSON_4Pages(b *testing.B) {
	raw1 := benchDataListJSON(500)
	results := make([]rawResult, 5)
	for pn := 2; pn <= 4; pn++ {
		results[pn] = rawResult{raw: raw1}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := assembleCirclesJSON(raw1, results, 4, nil); err != nil {
			b.Fatalf("assembleCirclesJSON 失败: %v", err)
		}
	}
}

// BenchmarkAppendPageRange_500 测偏移分页的逐字节扫描与裁剪（纯内存）。
func BenchmarkAppendPageRange_500(b *testing.B) {
	page := benchDataListJSON(500)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		first := true
		appendPageRange(&buf, page, &first, 0, 100, 200, 0)
	}
}

// BenchmarkRedactBodyThenTruncate_1MB 测脱敏对大输入的开销。
//
// 注意 契约：必须先脱敏再截断（先截断会让跨边界的敏感值泄漏）。
// 本 benchmark 用于判断该路径是否值得优化，不预设结论。
func BenchmarkRedactBodyThenTruncate_1MB(b *testing.B) {
	raw := benchUnifiedBody(benchDataListJSON(2000), 2000, 4)

	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = logx.RedactBodyThenTruncate(raw, 100)
	}
}

// BenchmarkDecodeDataList_500 测结构化解码路径（与 JSON 透传路径对比）。
func BenchmarkDecodeDataList_500(b *testing.B) {
	list := benchDataListJSON(500)
	resp := types.UnifiedResponse{DataList: (*json.RawMessage)(&list)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := types.DecodeDataList[types.CircleRecord](resp); err != nil {
			b.Fatalf("DecodeDataList 失败: %v", err)
		}
	}
}

// BenchmarkActivateSession_CacheHit 测长驻场景下每次业务调用必经的 session 缓存命中路径。
func BenchmarkActivateSession_CacheHit(b *testing.B) {
	srv := benchWarmupBizServer(b, []byte(`{"code":1,"msg":"成功"}`))
	c := benchClient(b, srv.URL)
	ctx := context.Background()

	if _, err := c.ActivateSession(ctx, benchToken); err != nil {
		b.Fatalf("预热失败: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.ActivateSession(ctx, benchToken); err != nil {
			b.Fatalf("ActivateSession 失败: %v", err)
		}
	}
}

// BenchmarkBuildBizHeaders 测每次请求都要构造的公共请求头 map。
func BenchmarkBuildBizHeaders(b *testing.B) {
	srv := benchBizServer(b, []byte(`{"code":1,"msg":"成功"}`))
	c := benchClient(b, srv.URL)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.bizHeaders(benchToken)
	}
}
