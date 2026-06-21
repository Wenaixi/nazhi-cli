package main

import (
	"bytes"
	"context"
	"encoding/base64"
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
	password := "TestPass123"

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:       jar,
		Timeout:   15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ctx := context.Background()

	// SSO 请求头
	ssoHeaders := map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		"Referer":          baseURL + "/uiStudentLogin/login",
		"Origin":           baseURL,
		"X-Requested-With": "XMLHttpRequest",
	}

	// 步骤1: InitSession
	fmt.Fprintln(os.Stderr, "=== 步骤1: InitSession ===")
	req1, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/uiStudentLogin/login", nil)
	for k, v := range ssoHeaders { req1.Header.Set(k, v) }
	resp1, _ := client.Do(req1)
	io.Copy(io.Discard, resp1.Body); resp1.Body.Close()
	fmt.Fprintf(os.Stderr, "状态: %d ✅\n", resp1.StatusCode)

	// 步骤2: GetSchoolID (必须在 FetchCaptcha 之前)
	fmt.Fprintln(os.Stderr, "\n=== 步骤2: GetSchoolID ===")
	h2 := map[string]string{}
	for k, v := range ssoHeaders { h2[k] = v }
	h2["Referer"] = baseURL + "/uiStudentLogin/login?userName=" + username
	json2, _ := json.Marshal(map[string]string{"key": ""})
	req2, _ := http.NewRequestWithContext(ctx, "POST",
		baseURL+"/teacher/auth/studentLogin/getSchoolIdByStudentNumber?userName="+username,
		bytes.NewReader(json2))
	for k, v := range h2 { req2.Header.Set(k, v) }
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := client.Do(req2)
	b2, _ := io.ReadAll(resp2.Body); resp2.Body.Close()
	fmt.Fprintf(os.Stderr, "状态: %d, 响应: %s\n", resp2.StatusCode, string(b2))

	// 步骤3: GET kaptcha.jpg（让服务端生成验证码状态）
	fmt.Fprintln(os.Stderr, "\n=== 步骤3: GET kaptcha.jpg ===")
	kaptchaURL := baseURL + "/kaptcha/kaptcha.jpg?t=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	req3, _ := http.NewRequestWithContext(ctx, "GET", kaptchaURL, nil)
	for k, v := range ssoHeaders { req3.Header.Set(k, v) }
	resp3, _ := client.Do(req3)
	b3, _ := io.ReadAll(resp3.Body); resp3.Body.Close()
	fmt.Fprintf(os.Stderr, "状态: %d, 图片大小: %d bytes\n", resp3.StatusCode, len(b3))

	// 输出 Base64 图片到 stdout（方便主人查看）
	b64 := base64.StdEncoding.EncodeToString(b3)
	fmt.Println(b64)

	// 等待主人输入验证码
	fmt.Fprint(os.Stderr, "\n👀 验证码图片已输出（打印到 stdout），请输入验证码: ")
	var captcha string
	fmt.Scanf("%s", &captcha)
	captcha = strings.TrimSpace(captcha)

	if captcha == "" {
		fmt.Fprintln(os.Stderr, "❌ 验证码不能为空")
		os.Exit(1)
	}

	// 步骤4: ValidateCaptcha
	fmt.Fprintf(os.Stderr, "\n=== 步骤4: ValidateCaptcha (验证码: %s) ===\n", captcha)
	json4, _ := json.Marshal(map[string]string{"captcha": captcha})
	req4, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/uiStudentLogin/validateCaptcha", bytes.NewReader(json4))
	for k, v := range ssoHeaders { req4.Header.Set(k, v) }
	req4.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp4, _ := client.Do(req4)
	b4, _ := io.ReadAll(resp4.Body); resp4.Body.Close()
	fmt.Fprintf(os.Stderr, "状态: %d, 响应: %s\n", resp4.StatusCode, string(b4))

	var captchaResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	json.Unmarshal(b4, &captchaResp)
	if captchaResp.Code != 1 {
		fmt.Fprintf(os.Stderr, "❌ 验证码校验失败: %s\n", captchaResp.Msg)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "✅ 验证码校验成功!")

	// 步骤5: Login
	fmt.Fprintln(os.Stderr, "\n=== 步骤5: POST 登录 ===")
	loginBody := map[string]string{
		"schoolId": "173",
		"username": username,
		"password": password,
		"captcha":  captcha,
	}
	json5, _ := json.Marshal(loginBody)
	req5, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/teacher/auth/studentLogin/validate", bytes.NewReader(json5))
	for k, v := range ssoHeaders { req5.Header.Set(k, v) }
	req5.Header.Set("Content-Type", "application/json; charset=UTF-8")
	resp5, _ := client.Do(req5)
	b5, _ := io.ReadAll(resp5.Body)

	location := resp5.Header.Get("Location")
	fmt.Fprintf(os.Stderr, "状态: %d\n", resp5.StatusCode)
	fmt.Fprintf(os.Stderr, "Location: %s\n", location)
	fmt.Fprintf(os.Stderr, "Body: %s\n", string(b5))
	resp5.Body.Close()

	if location != "" {
		token := extractToken(location)
		fmt.Fprintf(os.Stderr, "\n🎉 登录成功! Token:\n%s\n", token)
	} else {
		fmt.Fprintln(os.Stderr, "\n❌ 未获取到 Location 头")
		var result map[string]any
		json.Unmarshal(b5, &result)
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		enc.Encode(result)
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
