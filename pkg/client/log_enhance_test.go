package client_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestClientLogCarriesTraceID(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	c, _ := client.New(client.WithLogger(lg))
	ctx := logx.WithTraceID(context.Background(), "trace-xyz")
	c.LogDebugForTest(ctx, "hello %s", "world")
	if !strings.Contains(buf.String(), "trace-xyz") {
		t.Fatalf("want trace in log %q", buf.String())
	}
}

func TestLogDoesNotLeakToken(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	c, _ := client.New(client.WithLogger(lg))
	ctx := logx.WithTraceID(context.Background(), "tid")
	c.LogInfoForTest(ctx, "payload=%s", `{"token":"supersecrettoken123"}`)
	out := buf.String()
	if strings.Contains(out, "supersecrettoken123") {
		t.Fatalf("token leaked %q", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("want mask in %q", out)
	}
}
