// Package client 白盒测试：F8.6 ResponseHeaderTimeout 防服务端挂起。
//
// 修复动机：newHTTPClient 的 Transport 仅设 TLSHandshakeTimeout=10s，
// 没设 ResponseHeaderTimeout——服务端 TCP 握手完成后故意不写响应头时，
// net/http 默认无限等（只受 c.http.Timeout 控制），与期望的「15s 内必返回」不符。
//
// ponytail：end-to-end timeout 触发测试需 sleep 15s+，性价比低。
// 这里只断言 Transport.ResponseHeaderTimeout 配置正确（字段 > 0 且 <= 任务约束 15s），
// 生产行为由 net/http 团队保证，go test -timeout 自身就是 timeout 触发兜底。
package client

import (
	"net/http"
	"testing"
	"time"
)

// TestF86_ResponseHeaderTimeout_Configured 验证 newHTTPClient 的 Transport
// 配置了 ResponseHeaderTimeout 且在任务约束范围（> 0 且 <= 15s）。
func TestF86_ResponseHeaderTimeout_Configured(t *testing.T) {
	c := &Client{http: newHTTPClient()}

	tr, ok := c.http.Transport.(*http.Transport)
	if !ok || tr == nil {
		t.Fatal("c.http.Transport 必须是 *http.Transport")
	}

	if tr.ResponseHeaderTimeout == 0 {
		t.Fatal("Transport.ResponseHeaderTimeout 必须 > 0（防止服务端挂起无限等）")
	}
	if tr.ResponseHeaderTimeout > 15*time.Second {
		t.Errorf("Transport.ResponseHeaderTimeout = %v 超过任务约束 15s", tr.ResponseHeaderTimeout)
	}
	t.Logf("ResponseHeaderTimeout=%v（已配置）", tr.ResponseHeaderTimeout)
}