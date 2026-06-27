// Package client 内部白盒测试。
package client

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRequest_NoTokenLeakInDebugLog 回归测试：logDebug 不得把完整 token
// 写入日志，包括嵌入 Referer / Cookie / Authorization 等 header 的 token。
// 历史 bug：logDebug 只对 X-Auth-Token 截断到 16 字符，其他 header 完整打印。
// 但 session.go:37 会把完整 token 注入 Referer（如 `/homepage?token=<full-jwt>`），
// 触发完整 JWT 落 stderr → 凭据泄漏。
func TestRequest_NoTokenLeakInDebugLog(t *testing.T) {
	// 构造一个明显长于 16 字符的可识别 token
	const fullToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJURVNUMjAyNTAwMSIsImV4cCI6OTk5OTk5OTk5OX0.abcdefghijklmnop"

	// 捕获 slog Debug 输出
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// 模拟目标平台：返回 200 + 空 body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// 白盒构造 Client（避免触发 OCR/SSL 等无关初始化）
	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        nil,
	}

	// 模拟 session.go:37 行为：把完整 token 注入 Referer query string
	headers := map[string]string{
		"X-Auth-Token": fullToken,
		"Referer":      srv.URL + "/homepage?token=" + fullToken,
		"Cookie":       "JSESSIONID=abc; X-Auth-Token=" + fullToken,
	}

	// 触发 doRequest 的 logDebug 路径（响应内容无关紧要）
	_, _ = c.doRequest(context.Background(), http.MethodGet, srv.URL+"/test", nil, headers, "")

	logs := logBuf.String()
	if strings.Contains(logs, fullToken) {
		t.Errorf("完整 token 泄漏到日志中（应被脱敏）：\n--- LOGS ---\n%s--- END ---", logs)
	}
}
