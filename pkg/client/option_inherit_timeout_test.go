package client

import (
	"net/http"
	"testing"
	"time"
)

// TestWithTimeoutBeforeHTTPClient_PrevailsTimeout 验证 WithTimeout(15s) + WithHTTPClient(custom)
// 任意顺序 → 最终 httpClient.Timeout == 15s。该语义由 client.go:219-228 prevTimeout 继承
// 逻辑保证，避免 Option 声明顺序敏感性。
func TestWithTimeoutBeforeHTTPClient_PrevailsTimeout(t *testing.T) {
	custom := &http.Client{}
	c, err := New(
		WithTimeout(15*time.Second),
		WithHTTPClient(custom),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()
	if custom.Timeout != 15*time.Second {
		t.Fatalf("want custom.Timeout=15s got %v", custom.Timeout)
	}
}

// TestWithHTTPClientBeforeTimeout_PrevailsTimeout 验证顺序 2：WithHTTPClient + WithTimeout
// 仍继承正确超时。
func TestWithHTTPClientBeforeTimeout_PrevailsTimeout(t *testing.T) {
	custom := &http.Client{}
	c, err := New(
		WithHTTPClient(custom),
		WithTimeout(15*time.Second),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()
	if custom.Timeout != 15*time.Second {
		t.Fatalf("want custom.Timeout=15s got %v", custom.Timeout)
	}
}
