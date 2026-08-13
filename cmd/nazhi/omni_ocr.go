package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const (
	defaultOmniBaseURL = "https://api.siliconflow.cn/v1"
	defaultOmniModel   = "Qwen/Qwen3-Omni-30B-A3B-Instruct"
	omniOCRSystem      = "Output 4 alphanumeric characters."
)

// omniOCR 是 CLI 登录用的硅基流动 Qwen3-Omni 验证码识别器。
// 请求契约对齐 Nazhi-auto backend/internal/ai/siliconflow：
// POST /chat/completions，system 提示 + 仅 image_url 的 user 消息。
// 密钥不得写进仓库，只从运行时配置注入。
type omniOCR struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

func newOmniOCR(apiKey, baseURL, model string) *omniOCR {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOmniBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultOmniModel
	}
	return &omniOCR{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		model:   strings.TrimSpace(model),
		http:    &http.Client{Timeout: 45 * time.Second},
	}
}

func (o *omniOCR) Recognize(img []byte) (string, error) {
	if o == nil || o.apiKey == "" {
		return "", fmt.Errorf("Omni OCR 未配置 API Key")
	}
	if len(img) == 0 {
		return "", nil
	}
	body := map[string]any{
		"model": o.model,
		"messages": []any{
			map[string]any{"role": "system", "content": omniOCRSystem},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":      "image_url",
						"image_url": map[string]string{"url": omniDataURI(img)},
					},
				},
			},
		},
		"temperature": 0,
		"max_tokens":  256,
		"stream":      false,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("编码 Omni OCR 请求失败: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("构造 Omni OCR 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 Omni OCR 失败: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取 Omni OCR 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Omni OCR HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(out)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", fmt.Errorf("解析 Omni OCR 响应失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", nil
	}
	text := omniMessageText(parsed.Choices[0].Message.Content)
	cleaned := cleanCaptchaText(text)
	if !isValidCaptchaFormat(cleaned) {
		return "", nil
	}
	return cleaned, nil
}

func (o *omniOCR) Close() error { return nil }

func omniDataURI(img []byte) string {
	return "data:" + detectImageMIME(img) + ";base64," + base64.StdEncoding.EncodeToString(img)
}

func detectImageMIME(data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 3 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		return "image/gif"
	}
	if len(data) >= 4 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		return "image/webp"
	}
	return "image/png"
}

func omniMessageText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	default:
		return fmt.Sprint(v)
	}
}

func cleanCaptchaText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isValidCaptchaFormat(s string) bool {
	n := len(s)
	if n < 3 || n > 8 {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
