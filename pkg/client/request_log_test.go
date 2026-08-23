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

func TestHTTPLogLifecycleJSON(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			w.Write([]byte(`{"code":1,"msg":"ok"}`))
		case "/api/studentInfo/getMyInfo":
			w.Write([]byte(`{"code":1,"msg":"ok","returnData":{"name":"a","studentNumber":"TEST2025001"}}`))
		default:
			w.Write([]byte(`{"code":1,"msg":"ok","returnData":{}}`))
		}
	}))
	defer srv.Close()
	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithSSOBase(srv.URL), client.WithLogger(lg))
	ctx := logx.WithTraceID(context.Background(), "tid-123")
	_, _ = c.ActivateSession(ctx, "tok-abc")
	out := buf.String()
	if !strings.Contains(out, "tid-123") {
		t.Fatalf("missing trace in log %q", out)
	}
	// should contain status-level log (either → or ←)
	if !strings.Contains(out, "trace_id") {
		t.Fatalf("missing trace_id attr %q", out)
	}
}
