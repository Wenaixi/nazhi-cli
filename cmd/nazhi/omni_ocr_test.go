package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCleanCaptchaTextAndFormat(t *testing.T) {
	if got := cleanCaptchaText("4W 4K"); got != "4W4K" {
		t.Fatalf("清洗空格失败: %s", got)
	}
	if !isValidCaptchaFormat("4W4K") {
		t.Fatal("4W4K 应视为合法验证码")
	}
	if isValidCaptchaFormat("ab") {
		t.Fatal("过短验证码应拒绝")
	}
}

func TestOmniOCRFromEnvRequiresKey(t *testing.T) {
	t.Setenv("NAZHI_OCR_API_KEY", "")
	t.Setenv("SILICONFLOW_API_KEY", "")
	if omniOCRFromEnv() != nil {
		t.Fatal("未配置密钥时不应构造 Omni OCR")
	}
	t.Setenv("NAZHI_OCR_API_KEY", "sk-test-not-real")
	if omniOCRFromEnv() == nil {
		t.Fatal("配置密钥后应构造 Omni OCR")
	}
}

func TestOmniOCRRecognizeMatchesNazhiAutoContract(t *testing.T) {
	var gotAuth, gotModel, gotSystem string
	var gotImageOnly bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		gotModel, _ = payload["model"].(string)
		msgs, _ := payload["messages"].([]any)
		if len(msgs) >= 1 {
			sys, _ := msgs[0].(map[string]any)
			gotSystem, _ = sys["content"].(string)
		}
		if len(msgs) >= 2 {
			user, _ := msgs[1].(map[string]any)
			content, _ := user["content"].([]any)
			gotImageOnly = len(content) == 1
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"4W 4K"}}]}`))
	}))
	defer srv.Close()

	ocr := newOmniOCR("sk-test-not-real", srv.URL, "Qwen/Qwen3-Omni-30B-A3B-Instruct")
	text, err := ocr.Recognize([]byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0})
	if err != nil {
		t.Fatalf("Recognize 失败: %v", err)
	}
	if text != "4W4K" {
		t.Fatalf("应清洗为 4W4K，实际 %q", text)
	}
	if gotAuth != "Bearer sk-test-not-real" {
		t.Fatalf("鉴权头不符: %s", gotAuth)
	}
	if gotModel != "Qwen/Qwen3-Omni-30B-A3B-Instruct" {
		t.Fatalf("模型不符: %s", gotModel)
	}
	if gotSystem != omniOCRSystem {
		t.Fatalf("system prompt 应与 Nazhi-auto 一致，实际 %q", gotSystem)
	}
	if !gotImageOnly {
		t.Fatal("user 消息应仅含 image_url")
	}
}
