// Package client 内部白盒测试。
package client

import (
	"net/http"
	"testing"
	"time"
)

// TestNewCleanClient_SharesCustomTransport 验证用户注入自定义 Transport 时
// cleanClient 共享同一 Transport（保留连接池、代理、TLS 配置）。
func TestNewCleanClient_SharesCustomTransport(t *testing.T) {
	customRT := http.RoundTripper(http.DefaultTransport) // 用真实 DefaultTransport 当 stub
	c := &Client{
		http: &http.Client{Transport: customRT, Timeout: 5 * time.Second},
	}
	cc := newCleanClient(c)
	if cc.Transport != customRT {
		t.Error("cleanClient.Transport 应 == c.http.Transport（共享以复用连接池）")
	}
	if cc.Jar != nil {
		t.Error("cleanClient.Jar 必须 nil（防止 cookie 泄漏）")
	}
	if cc.Timeout != 5*time.Second {
		t.Errorf("cleanClient.Timeout 应 = 5s，实际 %v", cc.Timeout)
	}
}

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
