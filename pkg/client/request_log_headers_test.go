package client

import (
	"net/http"
	"testing"
)

// TestLogRequestHeaders_NilLogger_NoPanic 回归测试（F1）：
// logRequestHeaders 应当先检查 c.logger == nil，防止 nil pointer panic。
// 与 logDebug 的 nil 守卫（client.go:347）对称一致。
func TestLogRequestHeaders_NilLogger_NoPanic(t *testing.T) {
	c := &Client{logger: nil}
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}

	// 未修复前：c.logger.Enabled(...) 因 c.logger==nil 而 panic
	// 修复后应正常返回
	c.logRequestHeaders(req)
}
