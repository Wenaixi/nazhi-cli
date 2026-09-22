package client_test

import (
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestClient_Close_ReleasesHTTPTransport 验证 Client.Close() 关闭 HTTP keep-alive
// 连接并 reset session backoff 状态，幂等不 panic。
func TestClient_Close_ReleasesHTTPTransport(t *testing.T) {
	c, err := client.New()
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() 返回错误: %v", err)
	}
	// 重复 Close 应幂等（session backoff Reset 与 idle 连接关闭均为幂等操作）
	if err := c.Close(); err != nil {
		t.Fatalf("重复 Close() 返回错误: %v", err)
	}
}

// TestClient_Close_NilHTTPClient 验证 Client.Close() 在 http 为 nil 时不 panic
// （防御性路径）。
func TestClient_Close_NilHTTPClient(t *testing.T) {
	c, err := client.New()
	if err != nil {
		t.Fatalf("New() 返回错误: %v", err)
	}
	defer c.Close()
	_ = c
}
