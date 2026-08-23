package logx_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
)

func TestBoundary_ParseLevel_Edge(t *testing.T) {
	cases := []struct{ in string; want slog.Level; ok bool }{
		{"debug", slog.LevelDebug, true},
		{"DEBUG", slog.LevelDebug, true},
		{" debug ", slog.LevelDebug, true},
		{"info", slog.LevelInfo, true},
		{"INFO", slog.LevelInfo, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"WARNING", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
		{"", slog.LevelWarn, false},
		{" ", slog.LevelWarn, false},
		{"bogus", slog.LevelWarn, false},
		{"verbose", slog.LevelWarn, false},
	}
	for _, c := range cases {
		got, err := logx.ParseLevel(c.in)
		if c.ok && err != nil {
			t.Fatalf("ParseLevel(%q) want ok got err %v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("ParseLevel(%q) want err got ok", c.in)
		}
		if c.ok && got != c.want {
			t.Fatalf("ParseLevel(%q) want %v got %v", c.in, c.want, got)
		}
	}
}

func TestBoundary_ParseFormat_Edge(t *testing.T) {
	if got, _ := logx.ParseFormat(""); got != "text" {
		t.Fatalf("empty format should be text got %q", got)
	}
	if got, _ := logx.ParseFormat("  "); got != "text" {
		t.Fatalf("whitespace format should be text")
	}
	if got, _ := logx.ParseFormat("JSON"); got != "json" {
		t.Fatalf("case insensitive json want json got %q", got)
	}
	if got, _ := logx.ParseFormat("TEXT"); got != "text" {
		t.Fatalf("case insensitive text")
	}
	if _, err := logx.ParseFormat("xml"); err == nil {
		t.Fatal("xml should err")
	}
	if _, err := logx.ParseFormat("yaml"); err == nil {
		t.Fatal("yaml should err")
	}
}

func TestBoundary_NewTraceID_UniqueAndHex(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := logx.NewTraceID()
		if len(id) != 12 {
			t.Fatalf("trace len want 12 got %d %q", len(id), id)
		}
		for _, ch := range id {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
				t.Fatalf("non-hex trace %q", id)
			}
		}
		if seen[id] {
			t.Fatalf("duplicate trace %q", id)
		}
		seen[id] = true
	}
}

func TestBoundary_TraceContext_NilAndEmpty(t *testing.T) {
	if got := logx.TraceIDFrom(context.Background()); got != "" {
		t.Fatalf("background should have empty trace got %q", got)
	}
	if got := logx.TraceIDFrom(nil); got != "" {
		// context.WithValue panics on nil, but TraceIDFrom should handle nil gracefully (returns "")
		// Actually passing nil context will panic when calling Value on nil? Our impl does ctx.Value which on nil panics?
		// We expect it to handle? Currently it will panic. So we skip nil check - just verify not panic with background.
		t.Fatalf("nil ctx should not panic, got %q", got)
	}
}

func TestBoundary_RedactHeader_Edge(t *testing.T) {
	// sensitive case-insensitive
	cases := []struct{ k, v, wantContains string; wantNot string }{
		{"X-Auth-Token", "eyJhbGciOiJIUzI1NiJ9.payload", "***", "eyJhbGci"},
		{"x-auth-token", "short", "***", ""},
		{"Authorization", "Bearer abc123", "***", "Bearer"},
		{"token", "ab", "***", ""},
		{"password", "MySecretPass123", "***", "MySecretPass"},
		{"captcha", "ABCD", "***", "ABCD"},
		{"Content-Type", "application/json", "application/json", ""},
		{"Referer", "http://example.com/homepage?token=tok12345&other=1", "***", "tok12345"},
		{"Referer", "http://example.com/home", "http://example.com/home", ""},
		{"Referer", "http://example.com/TOKEN=abc", "***", ""}, // case-insensitive token=
	}
	for _, c := range cases {
		got := logx.RedactHeader(c.k, c.v)
		if !strings.Contains(got, c.wantContains) {
			t.Fatalf("RedactHeader(%q,%q) want contains %q got %q", c.k, c.v, c.wantContains, got)
		}
		if c.wantNot != "" && strings.Contains(got, c.wantNot) {
			t.Fatalf("RedactHeader(%q,%q) should not contain %q got %q", c.k, c.v, c.wantNot, got)
		}
	}
	// short value mask
	if got := logx.RedactHeader("token", "ab"); got != "***" {
		t.Fatalf("short token should be *** got %q", got)
	}
	if got := logx.RedactHeader("token", "abcde"); got != "ab***de" {
		t.Fatalf("maskValue keep 2+2 want ab***de got %q", got)
	}
}

func TestBoundary_RedactBody_Edge(t *testing.T) {
	// JSON sensitive keys
	body := `{"token":"secret123","name":"alice","password":"p@ss","captcha":"ABCD","other":"keep"}`
	red := logx.RedactBody(body)
	for _, leak := range []string{"secret123", "p@ss", "ABCD"} {
		if strings.Contains(red, leak) {
			t.Fatalf("leak %q in %q", leak, red)
		}
	}
	if !strings.Contains(red, "***") {
		t.Fatalf("want mask")
	}
	if !strings.Contains(red, "alice") {
		t.Fatalf("non-sensitive should remain")
	}
	// case-insensitive key
	red2 := logx.RedactBody(`{"TOKEN":"s3cret"}`)
	if strings.Contains(red2, "s3cret") {
		t.Fatalf("case-insensitive TOKEN leak")
	}
	// URL token query
	red3 := logx.RedactBody("http://example.com/homepage?token=tok12345&x=1")
	if strings.Contains(red3, "tok12345") {
		t.Fatalf("url token leak in body %q", red3)
	}
	if !strings.Contains(red3, "token=***") {
		t.Fatalf("url token should be masked got %q", red3)
	}
	// truncation
	long := strings.Repeat("a", 500)
	redLong := logx.RedactBody(long)
	if len(redLong) != 259 { // 256 + "..."
		t.Fatalf("truncation want 259 got %d", len(redLong))
	}
	if !strings.HasSuffix(redLong, "...") {
		t.Fatalf("truncation should end with ...")
	}
	// empty
	if got := logx.RedactBody(""); got != "" {
		t.Fatalf("empty should stay empty got %q", got)
	}
	// no sensitive content should pass through (but may be truncated)
	plain := `{"name":"bob"}`
	if got := logx.RedactBody(plain); got != plain {
		t.Fatalf("plain should pass got %q", got)
	}
}

func TestBoundary_NewLogger_Edge(t *testing.T) {
	// no writers defaults to stderr (not panic)
	lg := logx.NewLogger(slog.LevelDebug, "text")
	if lg == nil {
		t.Fatal("logger nil")
	}
	// level filtering
	var buf bytes.Buffer
	lg2 := logx.NewLogger(slog.LevelWarn, "text", &buf)
	lg2.Debug("should not appear")
	if buf.Len() != 0 {
		t.Fatalf("warn level should filter debug got %q", buf.String())
	}
	lg2.Warn("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Fatalf("warn should appear")
	}
	// json format contains level field
	buf.Reset()
	lg3 := logx.NewLogger(slog.LevelDebug, "json", &buf)
	lg3.Info("hello")
	if !strings.Contains(buf.String(), "\"level\"") && !strings.Contains(buf.String(), "\"msg\"") {
		t.Fatalf("json format missing fields got %q", buf.String())
	}
	// multi-writer fans out to both
	var b1, b2 bytes.Buffer
	lg4 := logx.NewLogger(slog.LevelDebug, "text", &b1, &b2)
	lg4.Info("fanout")
	if !strings.Contains(b1.String(), "fanout") || !strings.Contains(b2.String(), "fanout") {
		t.Fatalf("multi-writer fanout failed b1=%q b2=%q", b1.String(), b2.String())
	}
}

func TestBoundary_MultiWriter_Concurrent(t *testing.T) {
	var b1, b2 bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "text", &b1, &b2)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			lg.Info("concurrent", slog.Int("n", n))
		}(i)
	}
	wg.Wait()
	// each buffer should have 100 lines (approx)
	if c := strings.Count(b1.String(), "concurrent"); c != 100 {
		t.Fatalf("b1 concurrent count want 100 got %d", c)
	}
	if c := strings.Count(b2.String(), "concurrent"); c != 100 {
		t.Fatalf("b2 concurrent count want 100 got %d", c)
	}
}

func TestBoundary_NewFileWriter_Edge(t *testing.T) {
	// create temp file and verify append
	dir := t.TempDir()
	p := filepath.Join(dir, "test.log")
	fw, err := logx.NewFileWriter(p)
	if err != nil {
		t.Fatalf("NewFileWriter err %v", err)
	}
	defer fw.Close()
	lg := logx.NewLogger(slog.LevelDebug, "text", fw)
	lg.Info("file-log-test")
	fw.Close()
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), "file-log-test") {
		t.Fatalf("file not written %q", string(data))
	}
	// append second write
	fw2, _ := logx.NewFileWriter(p)
	lg2 := logx.NewLogger(slog.LevelDebug, "text", fw2)
	lg2.Info("second-line")
	fw2.Close()
	data2, _ := os.ReadFile(p)
	if strings.Count(string(data2), "line") < 1 {
		t.Fatalf("append failed %q", string(data2))
	}
	// perm 0600 (Windows 下权限位不精确，跳过校验)
	info, _ := os.Stat(p)
	if info != nil && info.Mode().Perm() != 0o600 {
		// 在非 Windows 下才严格校验
		if os.PathSeparator == '/' {
			t.Fatalf("want 0600 got %o", info.Mode().Perm())
		}
	}
}

func TestBoundary_MultiWriter_OneFailsOthersStillWrite(t *testing.T) {
	// failing writer should not block other writer (after fix)
	var good bytes.Buffer
	fail := &failingWriter{}
	lg := logx.NewLogger(slog.LevelDebug, "text", &good, fail)
	lg.Info("test-fanout-error")
	if !strings.Contains(good.String(), "test-fanout-error") {
		t.Fatalf("good writer should still receive despite failing writer")
	}
}

type failingWriter struct{}
func (f *failingWriter) Write(p []byte) (int, error) { return 0, os.ErrPermission }