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
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func TestAuthLogDoesNotLeakCaptchaAndPassword(t *testing.T) {
	var buf bytes.Buffer
	lg := logx.NewLogger(slog.LevelDebug, "json", &buf)
	// mock server: handle all SSO paths
	mux := http.NewServeMux()
	mux.HandleFunc("/uiStudentLogin/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/kaptcha/kaptcha.jpg", func(w http.ResponseWriter, r *http.Request) {
		// 1x1 png minimal
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
	})
	mux.HandleFunc("/uiStudentLogin/validateCaptcha", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":1,"msg":"ok"}`))
	})
	mux.HandleFunc("/teacher/auth/studentLogin/getSchoolIdByStudentNumber", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":1,"msg":"ok","dataList":[{"school_id":"1","NAME":"Test"}]}`))
	})
	mux.HandleFunc("/teacher/auth/studentLogin/validate", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":1,"msg":"ok","returnData":{"token":"tok123","expiresIn":3600}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	fakeOCR := &fakeOCRForLog{result: "ABCD"}
	c, _ := client.New(client.WithSSOBase(srv.URL), client.WithBaseURL(srv.URL), client.WithLogger(lg), client.WithCustomOCR(fakeOCR))
	ctx := logx.WithTraceID(context.Background(), "tid-auth-test")
	_, _ = c.Login(ctx, types.LoginRequest{Username: "u", Password: "MySecretPass123", SchoolID: "1"})
	out := buf.String()
	if strings.Contains(out, "ABCD") {
		t.Fatalf("captcha leaked in log %q", out)
	}
	if strings.Contains(out, "MySecretPass123") {
		t.Fatalf("password leaked in log %q", out)
	}
}

type fakeOCRForLog struct{ result string }

func (f *fakeOCRForLog) Recognize(_ []byte) (string, error) { return f.result, nil }
func (f *fakeOCRForLog) Close() error                       { return nil }
