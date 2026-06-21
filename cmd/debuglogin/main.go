package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"
)

func main() {
	baseURL := "https://www.nazhisoft.com"
	username := "S1234567890"

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:       jar,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ctx := context.Background()

	// 公共 SSO 请求头
	headers := map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		"Referer":          baseURL + "/uiStudentLogin/login",
		"Origin":           baseURL,
		"X-Requested-With": "XMLHttpRequest",
	}

	// 步骤1: InitSession
	fmt.Fprintln(os.Stderr, "=== 步骤1: InitSession GET /uiStudentLogin/login ===")
	req1, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/uiStudentLogin/login", nil)
	for k, v := range headers { req1.Header.Set(k, v) }
	resp1, _ := client.Do(req1)
	io.Copy(io.Discard, resp1.Body); resp1.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d ✅\n", resp1.StatusCode)

	// 步骤2: GetSchoolID
	fmt.Fprintln(os.Stderr, "\n=== 步骤2: PostSchoolID ===")
	url2 := baseURL + "/teacher/auth/studentLogin/getSchoolIdByStudentNumber?userName=" + username
	h2 := map[string]string{}
	for k, v := range headers { h2[k] = v }
	h2["Referer"] = baseURL + "/uiStudentLogin/login?userName=" + username
	json2, _ := json.Marshal(map[string]string{"key": ""})
	req2, _ := http.NewRequestWithContext(ctx, "POST", url2, bytes.NewReader(json2))
	for k, v := range h2 { req2.Header.Set(k, v) }
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := client.Do(req2)
	b2, _ := io.ReadAll(resp2.Body); resp2.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d, Body: %s\n", resp2.StatusCode, string(b2))

	// 步骤3: GET kaptcha.jpg（这一步让服务端生成验证码状态）
	fmt.Fprintln(os.Stderr, "\n=== 步骤3: GET kaptcha.jpg ===")
	url3 := baseURL + "/kaptcha/kaptcha.jpg?t=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	req3, _ := http.NewRequestWithContext(ctx, "GET", url3, nil)
	for k, v := range headers { req3.Header.Set(k, v) }
	resp3, _ := client.Do(req3)
	io.Copy(io.Discard, resp3.Body); resp3.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d ✅\n", resp3.StatusCode)

	// 步骤4: ValidateCaptcha（用 UTF-8 charset）
	fmt.Fprintln(os.Stderr, "\n=== 步骤4: ValidateCaptcha ===")
	json4, _ := json.Marshal(map[string]string{"captcha": "4xwk"})
	req4, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/uiStudentLogin/validateCaptcha", bytes.NewReader(json4))
	for k, v := range headers { req4.Header.Set(k, v) }
	req4.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp4, _ := client.Do(req4)
	b4, _ := io.ReadAll(resp4.Body); resp4.Body.Close()
	fmt.Fprintf(os.Stderr, "Status: %d, Body: %s\n", resp4.StatusCode, string(b4))

	// 解析验证码结果
	var captchaResult struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.Unmarshal(b4, &captchaResult)
	if captchaResult.Code != 1 {
		fmt.Fprintf(os.Stderr, "\n❌ 验证码校验失败: code=%d msg=%s\n", captchaResult.Code, captchaResult.Msg)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✅ 验证码校验成功!")

	// 步骤5: Login POST
	fmt.Fprintln(os.Stderr, "\n=== 步骤5: Login POST ===")
	loginBody := map[string]string{
		"schoolId": "173",
		"username": username,
		"password": "TestPass123",
		"captcha":  "4xwk",
	}
	json5, _ := json.Marshal(loginBody)
	req5, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/teacher/auth/studentLogin/validate", bytes.NewReader(json5))
	for k, v := range headers { req5.Header.Set(k, v) }
	req5.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp5, _ := client.Do(req5)
	b5, _ := io.ReadAll(resp5.Body)

	location := resp5.Header.Get("Location")
	fmt.Fprintf(os.Stderr, "Status: %d\n", resp5.StatusCode)
	fmt.Fprintf(os.Stderr, "Location: %s\n", location)
	fmt.Fprintf(os.Stderr, "Body: %s\n", string(b5))
	resp5.Body.Close()

	if location != "" {
		token := extractToken(location)
		fmt.Fprintf(os.Stderr, "\n🎉 Token: %s\n", token)
		printJSON(map[string]any{
			"token":    token,
			"location": location,
		})
	} else {
		fmt.Fprintln(os.Stderr, "\n❌ 未获取到 Location 头，登录失败")
		var result map[string]any
		json.Unmarshal(b5, &result)
		printJSON(result)
	}
}

func extractToken(location string) string {
	idx := strings.Index(location, "token=")
	if idx == -1 { return "" }
	token := location[idx+6:]
	if ampIdx := strings.Index(token, "&"); ampIdx != -1 {
		token = token[:ampIdx]
	}
	return token
}

func printJSON(v any) {
	json.NewEncoder(os.Stdout).Encode(v)
}
