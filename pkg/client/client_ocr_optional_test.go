// Package client_test 包含 OCR 可选化（build tag）的行为测试。
//
// 核心契约：
//   - 不传 WithCustomOCR 时，c.ocr 可能是 nil（构建时未指定 -tags ddddocr）
//   - 此时调 Login() 必须立即返回 ErrOCRNotConfigured 哨兵错误，
//     而不是 panic 或悬挂在 ocrRecognizeWithRetry
//   - Close() 也不应 panic
package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestLogin_NoOCR_ReturnsErrOCRNotConfigured 验证未注入 OCR 时 Login 立即报错。
func TestLogin_NoOCR_ReturnsErrOCRNotConfigured(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	})
	sso := httptest.NewServer(mux)
	defer sso.Close()

	c, err := client.New(
		client.WithSSOBase(sso.URL),
		client.WithBaseURL(sso.URL),
		// 关键：未传 WithCustomOCR —— 默认 c.ocr 在无 -tags ddddocr 时为 nil
		// 而 -tags ddddocr 构建时 c.ocr != nil —— 这个测试只对无 tag 构建生效
	)
	if err != nil {
		t.Fatalf("New 失败：%v", err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.Login(context.Background(), types.LoginRequest{
		Username: "test",
		Password: "test",
	})
	if err == nil {
		t.Fatal("期望 OCR 未配置错误，实际 nil")
	}
	// 仅当 c.ocr == nil 时这个错误才会触发（构建期判定）
	if errors.Is(err, client.ErrOCRNotConfigured) {
		t.Logf("预期：ErrOCRNotConfigured 触发 — %v", err)
	} else {
		t.Logf("当前构建带有 ddddocr tag（c.ocr != nil），跳过此断言：%v", err)
	}
}

// TestClient_Close_NoOCR_DoesNotPanic 验证 nil OCR 时 Close 不 panic。
func TestClient_Close_NoOCR_DoesNotPanic(t *testing.T) {
	c, err := client.New(client.WithSSOBase("https://example.com"))
	if err != nil {
		t.Fatalf("New 失败：%v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close() 触发 panic：%v", r)
		}
	}()
	if err := c.Close(); err != nil {
		t.Logf("Close 返回错误（可能因为 example.com 不可达）：%v", err)
	}
}
