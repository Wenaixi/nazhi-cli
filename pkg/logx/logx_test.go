package logx_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestParseLevel(t *testing.T) {
	if got, _ := logx.ParseLevel("debug"); got != slog.LevelDebug {
		t.Fatalf("want debug got %v", got)
	}
	if got, _ := logx.ParseLevel("INFO"); got != slog.LevelInfo {
		t.Fatalf("want info")
	}
	if _, err := logx.ParseLevel("bogus"); err == nil {
		t.Fatal("want err")
	}
}

func TestParseFormat(t *testing.T) {
	if got, _ := logx.ParseFormat("json"); got != "json" {
		t.Fatalf("want json")
	}
	if got, _ := logx.ParseFormat("text"); got != "text" {
		t.Fatalf("want text")
	}
	if _, err := logx.ParseFormat("xml"); err == nil {
		t.Fatal("want err")
	}
}

func TestTraceIDContext(t *testing.T) {
	ctx := logx.WithTraceID(context.Background(), "abc123")
	if got := logx.TraceIDFrom(ctx); got != "abc123" {
		t.Fatalf("got %q", got)
	}
	if id := logx.NewTraceID(); len(id) < 8 {
		t.Fatalf("trace too short %q", id)
	}
}

func TestRedactHeaderAndBody(t *testing.T) {
	if v := logx.RedactHeader("X-Auth-Token", "eyJhbGciOiJIUzI1NiJ9.payload.signature"); !strings.Contains(v, "***") {
		t.Fatalf("want mask got %q", v)
	}
	if v := logx.RedactHeader("Content-Type", "application/json"); v != "application/json" {
		t.Fatalf("non-sensitive should pass")
	}
	body := `{"token":"secret123","name":"alice","password":"p@ss"}`
	red := logx.RedactBody(body)
	if strings.Contains(red, "secret123") || strings.Contains(red, "p@ss") {
		t.Fatalf("sensitive leaked %q", red)
	}
	if !strings.Contains(red, "***") {
		t.Fatalf("want mask")
	}
}

func TestNewLogger_JSONContainsTraceAttr(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	lg.Info("hello", slog.String("trace_id", "tid-001"))
	if !strings.Contains(buf.String(), "tid-001") {
		t.Fatalf("want trace in output %q", buf.String())
	}
}
