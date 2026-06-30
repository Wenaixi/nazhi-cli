// clean_client_fresh_clone_test.go 验证 F5.6 修复后的 newCleanClient 行为：
// 每次调用现场 Clone c.http.Transport，不再用 sync.Once 缓存。
//
// 修复前（B1）：c.cleanTransportInit.Do 缓存首次 Clone 结果，
// 之后 c.http.Transport 的运行时变更永远不会被感知。
// 修复后：每次 newCleanClient 现场 type assertion + t.Clone()，
// Clone 成本是 O(1) struct copy + 重置 idle conn pool，
// 远低于 TLS 握手——典型批量上传场景 N 张图耗时仍以握手为主，
// 单次 Clone 几乎不可测量。
package client

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestNewCleanClient_FreshCloneEachCall 验证每次 newCleanClient 调用
// 都返回新的 cloned Transport 实例（不再缓存）。
//
// 修复前 RED：sync.Once 缓存导致 50 次调用都拿到同一实例。
// 修复后 GREEN：每次现场 Clone，实例不同。
func TestNewCleanClient_FreshCloneEachCall(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns: 50,
	}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	cc1 := newCleanClient(c)
	t1 := cc1.Transport.(*http.Transport)
	if t1 == originalTransport {
		t.Fatal("必须 Clone 出独立 Transport，不应等于原 Transport")
	}

	cc2 := newCleanClient(c)
	t2 := cc2.Transport.(*http.Transport)
	if t2 == t1 {
		t.Errorf("F5.6 修复后每次应现场 Clone：首次=%p 二次=%p 应不相等", t1, t2)
	}

	cc3 := newCleanClient(c)
	t3 := cc3.Transport.(*http.Transport)
	if t3 == t1 || t3 == t2 {
		t.Errorf("F5.6 修复后每次应现场 Clone：三次=%p 不应等于前两次 %p / %p", t3, t1, t2)
	}
}

// TestNewCleanClient_PicksUpRuntimeTransportChange 验证运行时替换
// c.http.Transport 后，新 Transport 立即生效（修复前因 sync.Once 缓存
// 会粘住第一次的 Clone，永远感知不到变更）。
func TestNewCleanClient_PicksUpRuntimeTransportChange(t *testing.T) {
	originalTransport := &http.Transport{MaxIdleConns: 10}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	// 第一次调用：Clone 出 MaxIdleConns=10 的独立 Transport
	cc1 := newCleanClient(c)
	cloned1 := cc1.Transport.(*http.Transport)
	if cloned1.MaxIdleConns != 10 {
		t.Fatalf("首次 Clone 应保留 MaxIdleConns=10，实际 %d", cloned1.MaxIdleConns)
	}

	// 运行时把 c.http.Transport 换成另一个独立 Transport
	replacedTransport := &http.Transport{MaxIdleConns: 99}
	c.http.Transport = replacedTransport

	// 第二次调用：必须现场 Clone replacedTransport（不是 cached 的 cloned1）
	cc2 := newCleanClient(c)
	cloned2 := cc2.Transport.(*http.Transport)
	if cloned2 == cloned1 {
		t.Errorf("F5.6 修复后应感知运行时 Transport 变更，不应复用首次 Clone 实例")
	}
	if cloned2.MaxIdleConns != 99 {
		t.Errorf("应 Clone replacedTransport，期望 MaxIdleConns=99，实际 %d", cloned2.MaxIdleConns)
	}
	if cloned2 == replacedTransport {
		t.Errorf("Clone 必须产出独立实例，不应等于原 Transport")
	}
}

// TestNewCleanClient_ConcurrentFreshCloneSafe 验证并发调用 newCleanClient
// 时每次都现场 Clone，且共享原 Transport 配置。
// 不再依赖 sync.Once，因此也不再有缓存一致性约束——但并发安全由
// type assertion + t.Clone() 的「无共享写」语义天然保证。
func TestNewCleanClient_ConcurrentFreshCloneSafe(t *testing.T) {
	originalTransport := &http.Transport{MaxIdleConns: 50}
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
			tr, ok := cc.Transport.(*http.Transport)
			if !ok {
				t.Errorf("goroutine %d: 应 Clone 为 *http.Transport，实际 %T", idx, cc.Transport)
				return
			}
			results[idx] = tr
		}(i)
	}
	wg.Wait()

	// F5.6 修复后：每次现场 Clone，所有实例应互不相同
	first := results[0]
	seen := map[*http.Transport]struct{}{first: {}}
	for i, tr := range results[1:] {
		if tr == nil {
			continue
		}
		if _, dup := seen[tr]; dup {
			t.Errorf("goroutine %d: 与前序实例重复 %p（应为独立 Clone）", i+1, tr)
		}
		seen[tr] = struct{}{}
		if tr.MaxIdleConns != 50 {
			t.Errorf("goroutine %d: Clone 应保留 MaxIdleConns=50，实际 %d", i+1, tr.MaxIdleConns)
		}
	}
}

// TestNewCleanClient_PreservesConfigOnFreshClone 验证每次现场 Clone
// 都保留原 Transport 配置（MaxIdleConns 等）。
func TestNewCleanClient_PreservesConfigOnFreshClone(t *testing.T) {
	originalTransport := &http.Transport{MaxIdleConns: 100}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	for i := 0; i < 3; i++ {
		cc := newCleanClient(c)
		cleanTransport := cc.Transport.(*http.Transport)
		if cleanTransport.MaxIdleConns != 100 {
			t.Errorf("第 %d 次调用: Clone 应保留 MaxIdleConns=100，实际 %d", i, cleanTransport.MaxIdleConns)
		}
		if cleanTransport == originalTransport {
			t.Errorf("第 %d 次调用: Clone 必须产出独立实例", i)
		}
		if cc.Jar != nil {
			t.Errorf("第 %d 次调用: cleanClient.Jar 必须 nil", i)
		}
	}
}
