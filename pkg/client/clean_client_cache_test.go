// clean_client_cache_test.go 验证 newCleanClient 现场 Clone 的行为。
// 历史 bug：sync.Once 缓存一次 Transport，运行时变更永不感知。
// 现行为：每次 newCleanClient 现场 t.Clone()，不缓存。
//
// 约束：
// 1. Clone 出的 Transport 必须 ≠ 原 Transport（idle 池隔离）
// 2. 必须保留原 Transport 配置（MaxIdleConns 等）
// 3. 默认 Transport fallback 走 http.DefaultTransport
package client

import (
	"net/http"
	"testing"
	"time"
)

// TestNewCleanClient_ClonesHTTPTransportOnEachCall 验证每次调用都 Clone
// 出独立 Transport，且保留配置。
func TestNewCleanClient_ClonesHTTPTransportOnEachCall(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns: 50,
	}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	// 第一次调用
	cc1 := newCleanClient(c)
	t1 := cc1.Transport.(*http.Transport)
	if t1 == originalTransport {
		t.Fatal("必须 Clone 出独立 Transport")
	}
	if t1.MaxIdleConns != 50 {
		t.Errorf("Clone 应保留 MaxIdleConns=50，实际 %d", t1.MaxIdleConns)
	}
	if cc1.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil")
	}
	if cc1.Timeout != 30*time.Second {
		t.Errorf("cleanClient.Timeout 应上浮至最小 30s，实际 %v", cc1.Timeout)
	}
}

// TestNewCleanClient_EachCallGetsFreshClone 验证每次调用都返回新的 Clone，
// 不再跨调用复用同一实例。
func TestNewCleanClient_EachCallGetsFreshClone(t *testing.T) {
	originalTransport := &http.Transport{MaxIdleConns: 50}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}

	cc1 := newCleanClient(c)
	t1 := cc1.Transport.(*http.Transport)

	cc2 := newCleanClient(c)
	t2 := cc2.Transport.(*http.Transport)

	cc3 := newCleanClient(c)
	t3 := cc3.Transport.(*http.Transport)

	if t2 == t1 {
		t.Errorf("每次应现场 Clone：首次=%p 二次=%p", t1, t2)
	}
	if t3 == t1 || t3 == t2 {
		t.Errorf("每次应现场 Clone：三次=%p 不应等于前两次", t3)
	}
	if t1.MaxIdleConns != 50 || t2.MaxIdleConns != 50 || t3.MaxIdleConns != 50 {
		t.Error("每次 Clone 都应保留配置")
	}
}

// TestNewCleanClient_DefaultTransportNotCached 验证 Transport=nil 时
// 回退 Proxy:nil 的独立 Transport（不再是 http.DefaultTransport 本身，
// 防环境变量代理劫持 ）。
func TestNewCleanClient_DefaultTransportNotCached(t *testing.T) {
	c := &Client{
		http: &http.Client{Timeout: 10 * time.Second}, // Transport = nil
	}

	cc1 := newCleanClient(c)
	if cc1.Transport == http.DefaultTransport {
		t.Fatal("fallback 路径必须返回 Proxy:nil 的独立 Transport（不再是 http.DefaultTransport 本身，防代理劫持）")
	}
	if tr, ok := cc1.Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Fatalf("fallback Transport 应为 *http.Transport 且 Proxy=nil，实际 %T proxy=%v", cc1.Transport, (tr.Proxy != nil))
	}

	cc2 := newCleanClient(c)
	if cc2.Transport == http.DefaultTransport {
		t.Errorf("fallback 路径应持续返回独立 Transport，实际 %T", cc2.Transport)
	}
	if tr, ok := cc2.Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Errorf("cc2 fallback Transport 应为 *http.Transport 且 Proxy=nil，实际 %T proxy=%v", cc2.Transport, (tr.Proxy != nil))
	}
}

// TestNewCleanClient_CustomRoundTripperIsNotUsedForUpload 验证上传 clean client 不复用无法隔离的自定义传输器。
func TestNewCleanClient_CustomRoundTripperIsNotUsedForUpload(t *testing.T) {
	customRT := customRoundTripper{}
	c := &Client{
		http: &http.Client{Transport: customRT, Timeout: 5 * time.Second},
	}
	cc := newCleanClient(c)
	if cc.Transport == customRT {
		t.Error("上传 clean client 不应复用调用方自定义 RoundTripper")
	}
	if cc.Transport == http.DefaultTransport {
		t.Error("回退 Transport 不应是 http.DefaultTransport 本身（需 Proxy:nil 独立实例）")
	}
	if tr, ok := cc.Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Errorf("回退 Transport 应为 *http.Transport 且 Proxy=nil，实际 %T proxy=%v", cc.Transport, (tr.Proxy != nil))
	}
}

// TestNewCleanClient_FallbackToDefaultTransport 验证未注入 Transport 时
// 回退 Proxy:nil 独立 Transport。
func TestNewCleanClient_FallbackToDefaultTransport(t *testing.T) {
	c := &Client{
		http: &http.Client{Timeout: 10 * time.Second},
	}
	cc := newCleanClient(c)
	if cc.Transport == http.DefaultTransport {
		t.Error("c.http.Transport=nil 时应回退 Proxy:nil 独立 Transport（防代理劫持）")
	}
	if tr, ok := cc.Transport.(*http.Transport); !ok || tr.Proxy != nil {
		t.Errorf("回退 Transport 应为 *http.Transport 且 Proxy=nil，实际 %T", cc.Transport)
	}
	if cc.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil")
	}
}

// customRoundTripper 是测试用的自定义 RoundTripper 实现。
type customRoundTripper struct{}

func (customRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
