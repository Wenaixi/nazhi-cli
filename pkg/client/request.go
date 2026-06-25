package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

// drainAndClose 先 drain response body 再 Close，让 net/http 把连接归还 keep-alive 池。
//
// 关键不变量：未读完的 body 在 Close 时会强制关闭底层 TCP 连接，
// 下次请求必须重新 TLS 握手，keep-alive 失效。集中 helper 防止
// 3+ 处业务侧 verbatim defer（review-tdd F6 重构目标）。
//
// nil 安全：body 为 nil 时直接返回，避免 nil pointer panic。
func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

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

// buildRequest 构造 *http.Request，设置 Content-Type 和请求头。
func (c *Client) buildRequest(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Request, error) {
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
		return nil, fmt.Errorf("%w: 创建请求失败: %w", ErrNetwork, err)
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

	return req, nil
}

// doRequest 执行 HTTP 请求，自动设置请求头，返回响应体字节。
// headers 是可选的自定义请求头（合并到公共头之上）。
// contentType 为空时默认 application/json。
func (c *Client) doRequest(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) ([]byte, error) {
	req, err := c.buildRequest(ctx, method, url, body, headers, contentType)
	if err != nil {
		return nil, err
	}

	c.logDebug("→ %s %s", method, url)
	if c.logger.Enabled(context.Background(), slog.LevelDebug) {
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
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: 请求失败: %w", ErrNetwork, err)
	}
	defer func() {
		// 关键：先 drain body 再 close，让 net/http 把连接归还 keep-alive 池
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取响应体失败: %w", ErrNetwork, err)
	}

	c.logDebug("← %d (%d bytes)", resp.StatusCode, len(respBytes))
	return respBytes, nil
}

// doRequestWithResp 执行请求并返回 *http.Response（调用者负责关闭 Body）。
func (c *Client) doRequestWithResp(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Response, error) {
	req, err := c.buildRequest(ctx, method, url, body, headers, contentType)
	if err != nil {
		return nil, err
	}

	c.logDebug("→ %s %s", method, url)
	return c.http.Do(req)
}

// ─── 业务侧请求辅助 ───

// doBizGet 是业务侧"GET + drain + close + readall + status check"的标准 helper。
//
// 封装以下 4 步, 消除 session.go / auth.go 中的 boilerplate:
//  1. doRequestWithResp 发起请求 (返回 *http.Response, 调用方负责 body)
//  2. defer drain+close (让 net/http 把连接归还 keep-alive 池)
//  3. io.ReadAll 读 body
//  4. 检查 status 200, 非 200 返回包装错误
//
// 错误:
//   - 网络层失败 (连接拒绝/超时等) → 包装为 ErrNetwork
//   - 非 200 状态码 → 返回错误并附上 body 内容, 方便排查 server 端异常
//   - body 读取失败 → 包装为 ErrNetwork
//
// 注意: 这是"一次性消费" helper, 调用方拿到 []byte 后 body 已关闭。
// 如需保留 body 在函数返回后继续使用, 请直接用 doRequestWithResp。
func (c *Client) doBizGet(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	resp, err := c.doRequestWithResp(ctx, http.MethodGet, url, nil, headers, "")
	if err != nil {
		return nil, fmt.Errorf("%w: GET %s 失败: %w", ErrNetwork, url, err)
	}
	defer func() {
		// 关键：先 drain body 再 close，让 net/http 把连接归还 keep-alive 池
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取 GET %s 响应体失败: %w", ErrNetwork, url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return bodyBytes, fmt.Errorf("GET %s 返回非 200: %d body=%s", url, resp.StatusCode, string(bodyBytes))
	}
	return bodyBytes, nil
}
