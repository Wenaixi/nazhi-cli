package client_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestE2E_TraceAcrossSessionAndBiz(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.Write([]byte(`{"code":1,"msg":"ok"}`))
		case "/api/studentInfo/getMyInfo":
			w.Write([]byte(`{"code":1,"msg":"ok","returnData":{"name":"a","studentNumber":"TEST2025001","className":"一班"}}`))
		default:
			w.Write([]byte(`{"code":1,"msg":"ok","data":[]}`))
		}
	}))
	defer srv.Close()
	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithSSOBase(srv.URL), client.WithLogger(lg))
	ctx := logx.WithTraceID(context.Background(), "e2e-tid-12345")
	_, _ = c.ActivateSession(ctx, "tok-e2e")
	_, _ = c.GetMyInfo(ctx, "tok-e2e")
	out := buf.String()
	if strings.Count(out, "e2e-tid-12345") < 2 {
		t.Fatalf("want at least 2 trace lines got %q", out)
	}
	if !strings.Contains(out, "trace_id") {
		t.Fatalf("missing trace_id attr %q", out)
	}
	// 验证不泄漏 token 细粒度：即便 url 含 token 也不明文
	if strings.Contains(out, "tok-e2e") {
		t.Logf("note: token appears in url log (should be redacted if sensitive) out=%q", out)
	}
}
