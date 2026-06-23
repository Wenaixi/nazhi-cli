package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

// ─── HTTP 客户端构造 ───

// defaultSSOBase 是 SSO 域名默认值。
const defaultSSOBase = "https://www.nazhisoft.com"

// defaultBaseURL 是业务 API 域名默认值。
const defaultBaseURL = "http://139.159.205.146:8280"

// defaultUploadURL 是文件上传服务器默认地址。
const defaultUploadURL = "http://doc.nazhisoft.com"

// newHTTPClient 创建带独立 cookie jar 的 HTTP 客户端。
func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		// 不自动跟随重定向——我们需要手动从 Location 头提取 token
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ─── 公共请求头 ───

// ssoHeaders 返回 SSO 域名的公共请求头。
func (c *Client) ssoHeaders() map[string]string {
	return map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"User-Agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		"Referer":          c.ssoBaseURL + "/uiStudentLogin/login",
		"Origin":           c.ssoBaseURL,
		"X-Requested-With": "XMLHttpRequest",
	}
}

// bizHeaders 返回业务 API 的公共请求头。
func (c *Client) bizHeaders(token string) map[string]string {
	return map[string]string{
		"Accept":       "application/json, text/plain, */*",
		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		"X-Auth-Token": token,
		"Referer":      c.baseURL + "/homepage",
	}
}

// ─── HTTP 请求执行 ───

// doRequest 执行 HTTP 请求，自动设置请求头，返回响应体字节。
// headers 是可选的自定义请求头（合并到公共头之上）。
// contentType 为空时默认 application/json。
func (c *Client) doRequest(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			reqBody = bytes.NewReader(b)
		case string:
			reqBody = strings.NewReader(b)
		default:
			jsonBytes, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("序列化请求体失败: %w", err)
			}
			reqBody = bytes.NewReader(jsonBytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置 Content-Type
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 应用自定义请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	c.logDebug("→ %s %s", method, url)
	for k, v := range req.Header {
		if len(v) == 0 {
			continue
		}
		val := v[0]
		// 脱敏：所有 header value 长度 > 16 字符都截断到 16 字符
		// 防止 X-Auth-Token、Authorization、Cookie、Set-Cookie、Referer 中嵌入的 token
		// 等敏感信息泄漏到日志（参见 request_log_redact_test.go 回归测试）。
		if len(val) > 16 {
			c.logDebug("  Header: %s: %s...", k, val[:16])
		} else {
			c.logDebug("  Header: %s: %s", k, val)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNetwork, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	c.logDebug("← %d (%d bytes)", resp.StatusCode, len(respBytes))
	return respBytes, nil
}

// doRequestWithResp 执行请求并返回 *http.Response（调用者负责关闭 Body）。
func (c *Client) doRequestWithResp(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			reqBody = bytes.NewReader(b)
		case string:
			reqBody = strings.NewReader(b)
		default:
			jsonBytes, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("序列化请求体失败: %w", err)
			}
			reqBody = bytes.NewReader(jsonBytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	c.logDebug("→ %s %s", method, url)
	return c.http.Do(req)
}
