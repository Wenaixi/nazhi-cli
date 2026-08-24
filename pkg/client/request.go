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
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// drainAndClose 先 drain response body 再 Close，让 net/http 把连接归还 keep-alive 池。
//
// 关键不变量：未读完的 body 在 Close 时会强制关闭底层 TCP 连接，
// 下次请求必须重新 TLS 握手，keep-alive 失效。集中 helper 防止业务侧 verbatim defer。
//
// nil 安全：body 为 nil 时直接返回，避免 nil pointer panic。
func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

// levelForStatus 把 HTTP 状态码映射为日志级别。
func levelForStatus(code int) slog.Level {
	switch {
	case code >= 500:
		return slog.LevelError
	case code >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

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

// defaultSubmittedPageSize 是 GetSubmittedCircles 每页条数默认值。
// 实测 pageSize 服务端上限 500，设为 500 后 ≤500 条场景无需翻页。
// 超过 500 条时自动并发翻页（见 submitted.go）。
const defaultSubmittedPageSize = 500

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
//   - ResponseHeaderTimeout=15s：服务端 TCP 握手完成后故意不写响应头
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
			// 显式直连：平台是国内站点（nazhisoft.com），走系统代理反而引入
			// 假死连接风险（2026-08-24 代理断流导致 OCR 卡 120s 事故）。
			Proxy:                 nil,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			DisableCompression:    false,
			// Dial 阶段独立超时：代理/对端 SYN 无响应时快速失败，
			// 不占满上层 client.Timeout（假死连接事故的 SDK 侧加固）。
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
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

// doBizVoid 执行 fire-and-forget mutation 请求（不需要响应数据）。
// 与 doBizAndDecode 对称，统一 fire-and-forget mutation 出口。
//
// 注意：doBizAndDecode 内部已用 opName 包装错误，此处直接透传，不再二次包装。
func (c *Client) doBizVoid(ctx context.Context, token, opName, path string, method string, body any) error {
	_, err := c.doBizAndDecode(ctx, token, opName, path, method, body)
	return err
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
	var lastErr error
	for _, decode := range decoders {
		v, err := decode(*resp)
		if err == nil && v != nil {
			return v, nil
		}
		if err != nil {
			lastErr = err
			c.logDebugCtx(ctx, "%s doBizGetDecode fallback: %v", opName, err)
		}
	}
	// 用 sentinel ErrAllDecodersFailed 标记空结果，让调用方可用 errors.Is 精确识别，
	// 避免依赖字符串匹配（旧实现用 strings.Contains("所有解码器均失败") 脆弱）。
	// lastErr 为 nil 时为纯空成功；非 nil 时 Join 实际解码错误供排错。
	return nil, errors.Join(
		fmt.Errorf("%s: %w", opName, ErrAllDecodersFailed),
		lastErr,
	)
}

// logRequestHeaders 在 debug 级别输出请求头，敏感 header 自动脱敏。
func (c *Client) logRequestHeaders(ctx context.Context, req *http.Request) {
	if c.logger == nil {
		return
	}
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	for k, v := range req.Header {
		if len(v) == 0 {
			continue
		}
		red := logx.RedactHeader(k, v[0])
		c.logDebugCtx(ctx, "  Header: %s: %s", k, red)
	}
}

// do 构建请求、打印调试日志并执行 HTTP 调用，是 httpDo 和 rawDoWithResp 共享的核心。
func (c *Client) do(ctx context.Context, method, url string, body any, headers map[string]string, contentType string) (*http.Response, error) {
	req, err := c.buildRequest(ctx, method, url, body, headers, contentType)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	c.logDebugCtx(ctx, "→ %s %s", method, logx.RedactBody(url))
	c.logRequestHeaders(ctx, req)

	resp, err := c.http.Do(req)
	if err != nil {
		dur := time.Since(start)
		lvl := slog.LevelError
		if isTimeoutError(err) || errors.Is(err, context.Canceled) {
			lvl = slog.LevelWarn
		}
		c.logWithLevel(ctx, lvl, "✗ %s %s dur=%s err=%v", method, logx.RedactBody(url), dur, err)
		// 检测超时错误并用 ErrTimeout 包装。
		if isTimeoutError(err) {
			return nil, fmt.Errorf("%w: 请求 %s 失败: %w", ErrTimeout, url, err)
		}
		return nil, fmt.Errorf("%w: 请求 %s 失败: %w", ErrNetwork, url, err)
	}
	return resp, nil
}

// httpDo 执行 HTTP 请求，自动设置请求头，返回响应体字节。
// headers 是可选的自定义请求头（合并到公共头之上）。
// contentType 为空时默认 application/json。
//
// 非 2xx 状态码走 classifyHTTPStatus，返回 sentinel（ErrRateLimited /
// ErrServiceUnavailable / ErrInvalidResponse），与 doBizGet 行为对齐。
// 这样 doBizAndDecode 主路径能正确识别 401/429/5xx，而不会把 HTML/空 body
// 当成 JSON 解析错误或业务 code 拒绝。
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

	c.logWithLevel(ctx, levelForStatus(resp.StatusCode), "← %d %s (%d bytes) body=%s", resp.StatusCode, logx.RedactBody(url), len(respBytes), logx.RedactBody(logSafeBody(respBytes)))

	// 非 2xx：返回 sentinel，不把 body 当作成功 JSON 交给上层解码。
	// 2xx（含 201/204 等）视为传输成功，业务 code 仍由 DecodeResponse/CheckCode 判定。
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		sentinel := classifyHTTPStatus(resp.StatusCode, ErrInvalidResponse)
		return nil, fmt.Errorf("%w: %s %s 返回状态码 %d body=%s",
			sentinel, method, logx.RedactBody(url), resp.StatusCode, logx.RedactBody(logSafeBody(respBytes)))
	}
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
		return nil, fmt.Errorf("%w: 读取 GET %s 响应体失败: %w", ErrNetwork, logx.RedactBody(url), err)
	}
	if resp.StatusCode != http.StatusOK {
		// 按 StatusCode 切换 sentinel 包装，让 SDK 用户能通过 errors.Is 精确识别
		// 原因（限流 / 服务端异常 / HTTP 层错误）。doBizGet 是业务侧 GET helper，
		// sentinel 包装让 cmd 层和 SDK 用户统一 errors.Is 判定。
		sentinel := classifyHTTPStatus(resp.StatusCode, ErrInvalidResponse)
		return nil, fmt.Errorf("%w: GET %s 返回状态码 %d body=%s",
			sentinel, logx.RedactBody(url), resp.StatusCode, logx.RedactBody(logSafeBody(bodyBytes)))
	}
	return bodyBytes, nil
}
