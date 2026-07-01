package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// drainAndClose 先 drain response body 再 Close，让 net/http 把连接归还 keep-alive 池。
//
// 关键不变量：未读完的 body 在 Close 时会强制关闭底层 TCP 连接，
// 下次请求必须重新 TLS 握手，keep-alive 失效。集中 helper 防止
// 业务侧 verbatim defer（重构目标，同文件 httpDo/doBizGet
// 也复用此 helper）。
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

// classifyHTTPStatus 按 StatusCode 切换 sentinel 包装，消除 doBizGet 与 UploadFile
// HTTP 状态码分类的重复。
//
// defaultErr 用于非 429/5xx 的兜底（request.go: ErrInvalidResponse, file.go: ErrUploadRejected）。
func classifyHTTPStatus(code int, defaultErr error) error {
	switch {
	case code == http.StatusTooManyRequests:
		return ErrRateLimited
	case code >= 500 && code < 600:
		return ErrServiceUnavailable
	default:
		return defaultErr
	}
}

const defaultSSOBase = "https://www.nazhisoft.com"

// defaultBaseURL 是业务 API 域名默认值。
const defaultBaseURL = "http://139.159.205.146:8280"

// defaultUploadURL 是文件上传服务器默认地址。
const defaultUploadURL = "http://doc.nazhisoft.com"

// defaultUserAgent 是所有 HTTP 请求的 User-Agent 默认值。
const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

// noRedirect 禁用 HTTP 自动重定向。
// 包级复用，消除 3 处相同闭包（request.go / file.go / auth_test.go）。
var noRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

// newHTTPClient 创建带独立 cookie jar 和自定义 Transport 的 HTTP 客户端。
//
// Transport 配置要点：
//   - MaxIdleConnsPerHost=16：FetchTasks 8 路并发打到同一 biz host 时，
//     第 3-8 路无需重新握手（http.DefaultTransport 默认=2，导致 6/8 请求需 TCP+TLS 握手）。
//   - 共享 Transport 连接池：避免与 file.go cleanTransport 产生认知冲突，
//     两者各自独立的 idle 池，但配置对齐。
//   - TLSHandshakeTimeout=10s：TLS 慢握手场景（弱网 / 服务器负载高）不无限等待。
//   - ResponseHeaderTimeout=15s（F8.6）：服务端 TCP 握手完成后故意不写响应头
//     （慢响应头 / 假死 / DoS）时强制返回错误，避免无限等待。仅靠 c.http.Timeout
//     不够细粒度——TLSHandshakeTimeout 只覆盖握手阶段，ResponseHeader 阶段
//     net/http 默认无限等。
//   - 不设置 DisableCompression：平台返回 JSON 多数 < 1KB，压缩获益小但非有害。
func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		// 不自动跟随重定向——我们需要手动从 Location 头提取 token
		CheckRedirect: noRedirect,
		Transport: &http.Transport{
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			DisableCompression:    false,
		},
	}
}

// ─── 公共请求头 ───

// ssoHeaders 返回 SSO 域名的公共请求头。
func (c *Client) ssoHeaders() map[string]string {
	return map[string]string{
		"Accept":           "application/json, text/javascript, */*; q=0.01",
		"User-Agent":       defaultUserAgent,
		"Referer":          c.ssoBaseURL + "/uiStudentLogin/login",
		"Origin":           c.ssoBaseURL,
		"X-Requested-With": "XMLHttpRequest",
	}
}

// bizHeaders 返回业务 API 的公共请求头。
func (c *Client) bizHeaders(token string) map[string]string {
	return map[string]string{
		"Accept":       "application/json, text/plain, */*",
		"User-Agent":   defaultUserAgent,
		"X-Auth-Token": token,
		"Referer":      c.bizURL("/homepage"),
	}
}

// ─── HTTP 请求执行 ───

// buildRequest 构造 *http.Request，设置 Content-Type 和请求头。
//
// body 参数支持以下类型：
//   - nil：不设置 body（用于 GET 请求）
//   - io.Reader：直接透传为 body（multipart / 流式上传场景）
//   - []byte / string：按字节/字符串透传
//   - 其他任意类型：JSON 序列化后作为 body
//
// contentType 参数：当 body 是 io.Reader 时必须由调用方显式传入（multipart
// 场景下服务端依赖 boundary 解析 body），其他场景下若为空则默认 application/json。
//
// 增加 io.Reader 分支，使 UploadFile 等 multipart 场景
// 能复用本 helper，消除特例路径。
func (c *Client) buildRequest(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Request, error) {
	var reqBody io.Reader
	if body != nil {
		switch b := body.(type) {
		case io.Reader:
			// io.Reader 直接透传，调用方负责构造（multipart / 流式场景）
			reqBody = b
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

// doBizAndDecode 封装业务请求的"预热 session → httpDo → DecodeResponse → CheckCode"公共管线。
//
// 参数：
//   - ctx: 上下文
//   - token: X-Auth-Token
//   - opName: 操作名称（用于错误消息前缀，如 "GetSchoolID"）
//   - path: 业务 API 路径（如 "/api/test"），经 c.bizURL() 拼接完整 URL
//   - method: HTTP 方法
//   - body: 请求体（nil 或任意可 JSON 序列化类型）
//
// 返回：
//   - *types.UnifiedResponse: 通过 CheckCode 确认 code=1 的统一响应体
//   - error: 网络错误 / 响应解析错误 / 业务拒绝
//
// 可被 doBizGet 语义替代的调用点（GET + 无 body）可直接用现有 doBizGet 或本函数。
// POST + body 场景是本函数的主要受益者。
func (c *Client) doBizAndDecode(ctx context.Context, token, opName, path, method string, body any) (*types.UnifiedResponse, error) {
	if _, err := c.ActivateSession(ctx, token); err != nil {
		return nil, fmt.Errorf("%s 预热 session 失败: %w", opName, err)
	}
	headers := c.bizHeaders(token)

	bodyBytes, err := c.httpDo(ctx, method, c.bizURL(path), body, headers, "")
	if err != nil {
		return nil, fmt.Errorf("%s 请求失败: %w", opName, err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("%s 响应解析失败: %w", opName, err)
	}

	if err := types.CheckCode(resp); err != nil {
		return nil, errors.Join(ErrBusinessRejected, fmt.Errorf("%s失败: %w", opName, err))
	}
	return &resp, nil
}

// doBizGetDecode 封装 GET 请求的"预热 session → httpDo → DecodeResponse → CheckCode → 类型安全解码"管线。
//
// 参数：
//   - c: Client 实例
//   - ctx: 上下文
//   - token: X-Auth-Token
//   - opName: 操作名称（用于错误消息前缀）
//   - path: 业务 API 路径（如 "/api/test"），经 c.bizURL() 拼接完整 URL
//   - decoders: 一个或多个解码函数，按顺序尝试，第一个成功的结果返回
//     所有解码器均失败时返回错误。
//
// 典型用法（单解码器）：
//
//	result, err := doBizGetDecode[types.UserInfo](c, ctx, token, "GetMyInfo", "/path",
//	    types.DecodeReturnData[types.UserInfo],
//	)
//
// 带回退链的用法：
//
//	result, err := doBizGetDecode[types.UserInfo](c, ctx, token, "GetMyInfo", "/path",
//	    types.DecodeReturnData[types.UserInfo],
//	    types.DecodeDataMap[types.UserInfo],
//	)
func doBizGetDecode[T any](c *Client, ctx context.Context, token, opName, path string, decoders ...func(types.UnifiedResponse) (*T, error)) (*T, error) {
	resp, err := c.doBizAndDecode(ctx, token, opName, path, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	for _, decode := range decoders {
		v, err := decode(*resp)
		if err == nil && v != nil {
			return v, nil
		}
		if err != nil {
			c.logDebug("%s doBizGetDecode fallback: %v", opName, err)
		}
	}
	return nil, fmt.Errorf("%s: 所有解码器均失败", opName)
}

// logRequestHeaders 在 debug 级别输出请求头，值长度 > 16 字符时自动截断脱敏。
func (c *Client) logRequestHeaders(req *http.Request) {
	if c.logger == nil {
		return
	}
	if !c.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	for k, v := range req.Header {
		if len(v) == 0 {
			continue
		}
		val := v[0]
		if len(val) > 16 {
			c.logDebug("  Header: %s: %s...", k, val[:16])
		} else {
			c.logDebug("  Header: %s: %s", k, val)
		}
	}
}

// do 是 httpDo 和 rawDoWithResp 共享的构建+打印+执行核心。
// 提取自 cleanup-httpDo：消除 ~8 行重复 boilerplate（buildRequest + logDebug + logRequestHeaders + Do + 错误包装）。
func (c *Client) do(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Response, error) {
	req, err := c.buildRequest(ctx, method, url, body, headers, contentType)
	if err != nil {
		return nil, err
	}

	c.logDebug("→ %s %s", method, url)
	c.logRequestHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		// A1 修复：检测超时错误并用 ErrTimeout 包装。
		// 让 SDK 用户能 errors.Is(err, ErrTimeout) 区分「超时」vs「连不上」。
		if isTimeoutError(err) {
			return nil, fmt.Errorf("%w: 请求 %s 失败: %w", ErrTimeout, url, err)
		}
		return nil, fmt.Errorf("%w: 请求 %s 失败: %w", ErrNetwork, url, err)
	}
	return resp, nil
}

// httpDo 执行 HTTP 请求，自动设置请求头，返回响应体字节。
// 改名自 doRequest，降级为内部私有。
// headers 是可选的自定义请求头（合并到公共头之上）。
// contentType 为空时默认 application/json。
func (c *Client) httpDo(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) ([]byte, error) {
	resp, err := c.do(ctx, method, url, body, headers, contentType)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取响应体失败: %w", ErrNetwork, err)
	}

	c.logDebug("← %d (%d bytes)", resp.StatusCode, len(respBytes))
	return respBytes, nil
}

// rawDoWithResp 执行请求并返回 *http.Response（调用者负责关闭 Body）。
func (c *Client) rawDoWithResp(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Response, error) {
	resp, err := c.do(ctx, method, url, body, headers, contentType)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ─── 业务侧请求辅助 ───

// doBizGet 是业务侧"GET + drain + close + readall + status check"的标准 helper。
//
// 封装以下 4 步, 消除 session.go / auth.go 中的 boilerplate:
//  1. rawDoWithResp 发起请求 (返回 *http.Response, 调用方负责 body)
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
// 如需保留 body 在函数返回后继续使用, 请直接用 rawDoWithResp。
func (c *Client) doBizGet(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	resp, err := c.rawDoWithResp(ctx, http.MethodGet, url, nil, headers, "")
	if err != nil {
		return nil, err
	}
	defer drainAndClose(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: 读取 GET %s 响应体失败: %w", ErrNetwork, url, err)
	}
	if resp.StatusCode != http.StatusOK {
		// G2 修复：按 StatusCode 切换 sentinel 包装，让 SDK 用户能通过
		// errors.Is 精确识别原因（限流 / 服务端异常 / HTTP 层错误）。
		//
		// 分发规则：
		//   - 429 → ErrRateLimited（限流，SDK 用户退避后重试，可用 Retry-After）
		//   - 5xx → ErrServiceUnavailable（服务端临时不可用，指数退避）
		//   - 其他 4xx → ErrInvalidResponse（HTTP 协议层错误，区别于业务 code=0）
		//
		// 与 F9.2 sentinel 配对，doBizGet 是业务侧 GET helper（非 session/login 场景），
		// 这里的 sentinel 包装让 cmd 层和 SDK 用户统一 errors.Is 判定。
		var sentinel error
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			sentinel = ErrRateLimited
		case resp.StatusCode >= 500 && resp.StatusCode < 600:
			sentinel = ErrServiceUnavailable
		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			sentinel = ErrInvalidResponse
		default:
			// 3xx 等意外状态码（不应出现在 CheckRedirect=noRedirect 配置下）
			sentinel = ErrInvalidResponse
		}
		return nil, fmt.Errorf("%w: GET %s 返回状态码 %d body=%s",
			sentinel, url, resp.StatusCode, logSafeBody(bodyBytes))
	}
	return bodyBytes, nil
}

// isTimeoutError 检测错误是否为超时相关。
// 检查 ctx.DeadlineExceeded、*url.Error 超时、以及 net.OpErr 超时。
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}
	return false
}
