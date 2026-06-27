// Package client 内部白盒测试。
package client

import (
	"net/http"
	"testing"
	"time"
)

// TestNewCleanClient_ClonesHTTPTransport 验证当用户注入 *http.Transport 时，
// cleanClient 复制（Clone）一份独立 Transport，避免共享 idle 连接池。
// 契约：原实现共享 c.http.Transport，Client.Close() 的
// CloseIdleConnections 会清空业务 Client 的 idle 连接池导致后续业务请求
// 强制重连 TLS。修复后用 (*http.Transport).Clone() 隔离 idle 池。
func TestNewCleanClient_ClonesHTTPTransport(t *testing.T) {
	originalTransport := &http.Transport{
		MaxIdleConns: 50,
	}
	c := &Client{
		http: &http.Client{Transport: originalTransport, Timeout: 5 * time.Second},
	}
	cc := newCleanClient(c)
	cleanTransport, ok := cc.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("cleanClient.Transport 应为 *http.Transport 类型，实际 %T", cc.Transport)
	}
	if cleanTransport == originalTransport {
		t.Error("cleanClient.Transport 必须 Clone 出独立实例，不应共享原 Transport")
	}
	// Clone 保留配置（MaxIdleConns 等）
	if cleanTransport.MaxIdleConns != 50 {
		t.Errorf("Clone 应保留 MaxIdleConns 配置，实际 %d", cleanTransport.MaxIdleConns)
	}
	if cc.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil（防止 cookie 泄漏）")
	}
	if cc.Timeout != 5*time.Second {
		t.Errorf("cleanClient.Timeout 应 = 5s，实际 %v", cc.Timeout)
	}
}

// TestNewCleanClient_PassesThroughCustomRoundTripper 验证用户注入自定义
// RoundTripper（非 *http.Transport 类型）时 cleanClient 透传。
// 边界：自定义 RoundTripper 无法 Clone（接口无 Clone 方法），
// 直接透传。此时不存在 idle 池共享问题（自定义 RT 通常不维护连接池），
// 也不应 panic。
func TestNewCleanClient_PassesThroughCustomRoundTripper(t *testing.T) {
	customRT := customRoundTripper{}
	c := &Client{
		http: &http.Client{Transport: customRT, Timeout: 5 * time.Second},
	}
	cc := newCleanClient(c)
	if cc.Transport != customRT {
		t.Error("cleanClient 对自定义 RoundTripper 应透传，而非 Clone 或 panic")
	}
	if cc.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil（防止 cookie 泄漏）")
	}
}

// customRoundTripper 是测试用的自定义 RoundTripper 实现。
// 边界：自定义 RT 无法 Clone，newCleanClient 应直接透传而非 panic。
type customRoundTripper struct{}

func (customRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

// TestNewCleanClient_FallbackToDefaultTransport 验证未注入 Transport 时
// cleanClient 回退到 http.DefaultTransport（不丢失功能）。
func TestNewCleanClient_FallbackToDefaultTransport(t *testing.T) {
	c := &Client{
		http: &http.Client{Timeout: 10 * time.Second}, // Transport = nil
	}
	cc := newCleanClient(c)
	if cc.Transport != http.DefaultTransport {
		t.Error("c.http.Transport=nil 时 cleanClient 应回退到 http.DefaultTransport")
	}
	if cc.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil（防止 cookie 泄漏）")
	}
}
