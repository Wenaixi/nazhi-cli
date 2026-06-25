// clean_client_cache_test.go 验证 newCleanClient 跨调用复用同一个 cloned Transport。
//
// B1 修复契约：原实现每次 UploadFile 都 t.Clone() 一次 Transport，
// 50 张图 = 50 次 Clone，配置虽共享但每次构造新对象丢失累加的 idle pool。
// 修复后：cleanTransport 缓存在 Client 字段，首次 Clone 后复用。
//
// 约束（保留 F9 修复）：
//  1. clone 出的 Transport 必须 ≠ 原 Transport（idle pool 隔离）
//  2. 必须保留原 Transport 配置（MaxIdleConns 等）
//  3. 默认 Transport fallback 仍走 http.DefaultTransport（不被缓存）
package client

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestNewCleanClient_CachesClonedTransport 验证 newCleanClient 跨调用复用
// 同一个 cloned Transport 实例，不再每次 Clone。
//
// B1 修复前的 RED 表现：每次调用 newCleanClient 都返回一个新的 cloned Transport。
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
//
// B1 修复约束：缓存字段必须有并发保护（UploadFile 是公开 API，可并发调用）。
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
//
// B1 修复约束：Clone 只调用一次，配置保留与原 Transport 一致（MaxIdleConns 等）。
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
//
// 因为 http.DefaultTransport 是进程单例，多个 Client 共享缓存会引入隐性耦合。
// B1 修复约束：只有 Clone 出来的 Transport 才缓存，fallback 路径保持 stateless。
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
