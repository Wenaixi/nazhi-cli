// clean_client_cache_test.go 验证 newCleanClient 跨调用复用同一个 cloned Transport。
// 修复契约：原实现每次 UploadFile 都 t.Clone() 一次 Transport，
// 50 张图 = 50 次 Clone，配置虽共享但每次构造新对象丢失累加的 idle pool。
// 修复后：cleanTransport 缓存在 Client 字段，首次 Clone 后复用。
// 约束：
// 1. clone 出的 Transport 必须 ≠ 原 Transport（idle pool 隔离）
// 2. 必须保留原 Transport 配置（MaxIdleConns 等）
// 3. 默认 Transport fallback 仍走 http.DefaultTransport（不被缓存）
package client

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestNewCleanClient_CachesClonedTransport 验证 newCleanClient 跨调用复用
// 同一个 cloned Transport 实例，不再每次 Clone。
// 修复前的 RED 表现：每次调用 newCleanClient 都返回一个新的 cloned Transport。
// 修复后 GREEN：50 次调用都返回同一个 cachedTransport 实例。
func TestNewCleanClient_CachesClonedTransport(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns: 50,
	}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	// 第一次调用：触发 Clone 并缓存
	cc1 := newCleanClient(c)
	t1 := cc1.Transport.(*http.Transport)
	if t1 == originalTransport {
		t.Fatal("首次调用必须 Clone 出独立 Transport")
	}

	// 第二次调用：必须返回同一 cachedTransport 实例
	cc2 := newCleanClient(c)
	t2 := cc2.Transport.(*http.Transport)
	if t2 != t1 {
		t.Errorf("newCleanClient 跨调用必须复用同一 cloned Transport；首次=%p, 二次=%p", t1, t2)
	}

	// 第三次：仍然同一实例
	cc3 := newCleanClient(c)
	t3 := cc3.Transport.(*http.Transport)
	if t3 != t1 {
		t.Errorf("newCleanClient 应持续复用同一 cloned Transport；首次=%p, 三次=%p", t1, t3)
	}
}

// TestNewCleanClient_ConcurrentCachingSafe 验证并发调用 newCleanClient 时的
// 缓存安全性：sync.Once 或 sync.Mutex 保护下，N goroutine 并发调用都拿到
// 同一 cachedTransport 实例，不会出现 race 或重复 Clone。
// 约束：缓存字段必须有并发保护（UploadFile 是公开 API，可并发调用）。
func TestNewCleanClient_ConcurrentCachingSafe(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns: 50,
	}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	results := make([]*http.Transport, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cc := newCleanClient(c)
			results[idx] = cc.Transport.(*http.Transport)
		}(i)
	}
	wg.Wait()

	// 所有 goroutine 必须拿到同一个 cloned Transport 实例
	first := results[0]
	for i, tr := range results {
		if tr != first {
			t.Errorf("goroutine %d 拿到不同 Transport 实例: %p != %p", i, tr, first)
		}
	}
}

// TestNewCleanClient_PreservesConfigAfterCaching 验证缓存后 Transport 配置仍正确。
// 约束：Clone 只调用一次，配置保留与原 Transport 一致（MaxIdleConns 等）。
func TestNewCleanClient_PreservesConfigAfterCaching(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns: 100,
		// 注意：Proxy/IdleConnTimeout 等深 clone 字段不强求, 我们只测基本配置
	}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	cc := newCleanClient(c)
	cleanTransport := cc.Transport.(*http.Transport)

	if cleanTransport.MaxIdleConns != 100 {
		t.Errorf("Clone 应保留 MaxIdleConns=100，实际 %d", cleanTransport.MaxIdleConns)
	}
	if cleanTransport == originalTransport {
		t.Error("Clone 必须产出独立实例")
	}
	if cc.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil")
	}

	// 第二次调用也保留配置
	cc2 := newCleanClient(c)
	cleanTransport2 := cc2.Transport.(*http.Transport)
	if cleanTransport2.MaxIdleConns != 100 {
		t.Errorf("二次调用 Transport MaxIdleConns=100，实际 %d", cleanTransport2.MaxIdleConns)
	}
	if cleanTransport2 != cleanTransport {
		t.Error("二次调用必须复用同一实例")
	}
}

// TestNewCleanClient_DefaultTransportNotCached 验证原 Client 没注入 Transport 时
// （走 http.DefaultTransport fallback），不应被缓存为客户端的字段。
// 因为 http.DefaultTransport 是进程单例，多个 Client 共享缓存会引入隐性耦合。
// 约束：只有 Clone 出来的 Transport 才缓存，fallback 路径保持 stateless。
func TestNewCleanClient_DefaultTransportNotCached(t *testing.T) {
	c := &Client{
		http: &http.Client{Timeout: 10 * time.Second}, // Transport = nil → DefaultTransport
	}

	cc1 := newCleanClient(c)
	if cc1.Transport != http.DefaultTransport {
		t.Fatalf("Transport=nil 应回退 http.DefaultTransport，实际 %T", cc1.Transport)
	}

	// 第二次调用仍走 DefaultTransport（不应 panic 或缓存错误值）
	cc2 := newCleanClient(c)
	if cc2.Transport != http.DefaultTransport {
		t.Errorf("fallback 路径应持续返回 DefaultTransport，实际 %T", cc2.Transport)
	}
}

// TestNewCleanClient_FallbackPathNoCloneAfterFirst 验证 sync.Once.Do 走过
// fallback 分支（自定义 RoundTripper 或 nil Transport）后，sentinel 标记
// "已 fallback"，后续调用直接走 fallback 路径，不再每次重新 type-switch 判断
// 也不再误走 *http.Transport Clone 分支。
//
// F9 修复契约：
//   - 第一次调用：c.http.Transport 是自定义 RT（非 *http.Transport），
//     sync.Once.Do 走 default 分支，标记 fallback=true
//   - 第二次调用：即使有人运行时把 c.http.Transport 换成 *http.Transport，
//     sentinel 仍为 fallback=true，**绝不**误走 Clone 分支（会丢失自定义 RT 语义）
//   - 实际行为：fallback=true 时直接透传 c.http.Transport 或用 DefaultTransport
//
// 修复前 bug：第二次进入 if c.cleanTransport != nil → false, 然后 re-do type
// switch（虽然 sync.Once 不会再进 Do 闭包，但外层 if-else 每次都跑），导致运行时
// 替换 Transport 时行为不稳定。
func TestNewCleanClient_FallbackPathNoCloneAfterFirst(t *testing.T) {
	// 第一次：c.http.Transport 是自定义 RT（走 fallback）
	customRT := customRoundTripper{}
	c := &Client{
		http: &http.Client{Transport: customRT, Timeout: 5 * time.Second},
	}

	cc1 := newCleanClient(c)
	if cc1.Transport != customRT {
		t.Fatalf("首次调用应对自定义 RT 透传，实际 %T", cc1.Transport)
	}

	// 运行时把 Transport 换成 *http.Transport
	// F9 修复后：sentinel 已是 fallback=true，**不应**走 Clone 路径，
	// 也不应返回 stale 的 customRT（因为 fallback 路径每次都读 c.http.Transport 当前值）
	replacedTransport := &http.Transport{MaxIdleConns: 99}
	c.http.Transport = replacedTransport

	cc2 := newCleanClient(c)
	// F9 修复后行为：fallback 路径直接读 c.http.Transport 透传，
	// 不应再误入 Clone 分支（Clone 出的 Transport 不会等于 replacedTransport，
	// 也不应等于 customRT）
	if cc2.Transport == customRT {
		t.Errorf("fallback 路径应读 c.http.Transport 当前值，不应缓存旧 customRT")
	}
	// 行为契约：sentinel 标记 fallback=true 后，透传 c.http.Transport 当前值
	// （即 replacedTransport）
	if cc2.Transport != replacedTransport {
		t.Errorf("fallback sentinel 路径应透传 c.http.Transport 当前值，期望 %p, 实际 %p",
			replacedTransport, cc2.Transport)
	}
}

// TestNewCleanClient_NilTransportFallbackNoClone 验证 c.http.Transport=nil
// 走过 sync.Once fallback 后，sentinel 标记 fallback=true，后续即使 c.http.Transport
// 变成 *http.Transport 也不应走 Clone 分支（避免 sentinel 状态泄漏）。
func TestNewCleanClient_NilTransportFallbackNoClone(t *testing.T) {
	// 第一次：c.http.Transport = nil（走 fallback）
	c := &Client{
		http: &http.Client{Timeout: 5 * time.Second}, // Transport = nil
	}

	cc1 := newCleanClient(c)
	if cc1.Transport != http.DefaultTransport {
		t.Fatalf("首次调用 Transport=nil 应回退 http.DefaultTransport，实际 %T", cc1.Transport)
	}

	// 运行时把 Transport 换成 *http.Transport
	// F9 修复后：sentinel 标记 fallback=true，**不应**走 Clone 分支
	replacedTransport := &http.Transport{MaxIdleConns: 99}
	c.http.Transport = replacedTransport

	cc2 := newCleanClient(c)
	// F9 修复后行为：sentinel=fallback, 透传 c.http.Transport 当前值
	if cc2.Transport != replacedTransport {
		t.Errorf("fallback sentinel 后应透传 c.http.Transport 当前值，期望 %p, 实际 %p",
			replacedTransport, cc2.Transport)
	}
}
