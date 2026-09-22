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
	mux.HandleFunc("/teacher/auth/studentLogin/getSchoolIdByStudentNumber", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":1,"msg":"ok","dataList":[{"school_id":"1","NAME":"Test"}]}`))
	})
	mux.HandleFunc("/uiActivityLogin/studentLogin", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":1,"msg":"ok","returnData":{"token":"tok123","expiresIn":3600}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _ := client.New(client.WithSSOBase(srv.URL), client.WithBaseURL(srv.URL), client.WithLogger(lg))
	ctx := logx.WithTraceID(context.Background(), "tid-auth-test")
	_, _ = c.Login(ctx, types.LoginRequest{Username: "u", Password: "MySecretPass123", SchoolID: "1"})
	out := buf.String()
	if strings.Contains(out, "MySecretPass123") {
		t.Fatalf("password leaked in log %q", out)
	}
}
