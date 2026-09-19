package client_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

// 捕获 logger：把 New() 的告警写进 bytes.Buffer，供断言。
func captureWarnLogger() (*bytes.Buffer, *slog.Logger) {
	var buf bytes.Buffer
	return &buf, logx.NewLogger(slog.LevelWarn, "json", &buf)
}

// TestNew_WarnsOnPlaintextBaseURL 锁定 cli #10（P2 升级）：业务 API 与上传服务器
// 默认明文 HTTP（defaultBaseURL="http://139.159.205.146:8280"），且携带
// X-Auth-Token——中间人可直接截获 token 与 PII。README 声称有「New 非 HTTPS
// WARN」，但当前代码零告警。修复：New() 对 baseURL/uploadURL 为 http:// 时
// 输出 Warn（致调用方可见此风险）。
func TestNew_WarnsOnPlaintextBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	// srv.URL 是 http:// —— 用它触发明文告警（也顺带验证测试基建不依赖真实地址）

	buf, lg := captureWarnLogger()

	c, err := client.New(client.WithLogger(lg), client.WithBaseURL(srv.URL), client.WithUploadURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	out := buf.String()
	if !strings.Contains(out, "http") || !strings.Contains(out, "明文") {
		t.Fatalf("New() 对明文 baseURL 应输出明文告警，实际日志: %q", out)
	}
}

// TestNew_NoWarnOnHTTPSBaseURL 对照：https 的 baseURL 不应触发明文告警（防止修复过宽）。
func TestNew_NoWarnOnHTTPSBaseURL(t *testing.T) {
	buf, lg := captureWarnLogger()

	// http:// 是 srv.URL 的形态；这里直接传 https 假地址（不发起请求，只触发 New 的
	// 预解析与告警判断，不需要真实可达）。
	c, err := client.New(client.WithLogger(lg), client.WithBaseURL("https://api.example.com"), client.WithUploadURL("https://up.example.com"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	if out := buf.String(); strings.Contains(out, "明文") {
		t.Fatalf("https baseURL 不应触发明文告警，实际日志: %q", out)
	}
}
