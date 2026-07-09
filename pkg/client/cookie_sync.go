package client

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// syncCookieToken 将 JWT token 同步到 HTTP cookie jar 中（X-Auth-Token）。
// 业务 API 通过 cookie 鉴权而非 Authorization 头。
//
// F6 优化：baseURL 在 New() 阶段已预解析到 c.baseURLParsed，避免每次调用 url.Parse。
// 若直接构造 Client（绕过 New()），则懒解析一次并缓存回 c.baseURLParsed。
//
// F3 修复：c.baseURLParsed 改 atomic.Pointer[url.URL]，所有访问原子化。
// 修复前用 *url.URL + sync.Mutex 仍被 race detector 报警——
// url.Parse 内部对返回的 *url.URL 字段写入与 jar.SetCookies 的字段读取
// 虽跨不同 goroutine 但共享同一 *url.URL，atomic.Pointer 让所有访问原子化解决。
func (c *Client) syncCookieToken(token string) error {
	if c.http == nil {
		return fmt.Errorf("syncCookieToken 失败: HTTP client 为 nil，无法同步 token 到 cookie")
	}
	jar, ok := c.http.Jar.(*cookiejar.Jar)
	if !ok {
		return fmt.Errorf("syncCookieToken 失败: HTTP client 的 Jar 不是 *cookiejar.Jar（实际类型 %T），X-Auth-Token 无法同步到 cookie。"+
			"修复：用 client.New() 默认 HTTP 客户端，或显式 &http.Client{Jar: cookiejar.New(nil)} 创建",
			c.http.Jar)
	}

	// 优先使用预解析的 baseURLParsed，否则懒解析一次并缓存。
	// Load / CompareAndSwap 全原子，热路径无锁；懒解析路径用 CAS 防重复解析。
	u := c.baseURLParsed.Load()
	if u == nil {
		if c.baseURL == "" {
			return fmt.Errorf("syncCookieToken 失败: base URL 为空，无法同步 token 到 cookie")
		}
		parsed, err := url.Parse(c.baseURL)
		if err != nil {
			return fmt.Errorf("syncCookieToken 失败: 解析 base URL %q 出错: %w", c.baseURL, err)
		}
		if c.baseURLParsed.CompareAndSwap(nil, parsed) {
			u = parsed
		} else {
			// 输给另一 goroutine 的 CAS，使用赢家写入的 url.URL
			u = c.baseURLParsed.Load()
		}
	}

	jar.SetCookies(u, []*http.Cookie{{
		Name:  "X-Auth-Token",
		Value: token,
		Path:  "/",
	}})
	c.logDebug("X-Auth-Token 已同步到 cookie jar（%s）", c.baseURL)
	return nil
}

// warnSyncCookieToken 尝试同步 token 到 cookie，失败时仅 warn。
func (c *Client) warnSyncCookieToken(token, label string) {
	if err := c.syncCookieToken(token); err != nil {
		if c.logger != nil {
			c.logger.Warn("Login "+label+" 后同步 token 到 cookie 失败", "err", err.Error())
		}
	}
}

// buildLoginResponse 构建 LoginResponse，内部调用 warnSyncCookieToken。
//
// v1.0.0 精简：删除 RawData 字段（json:"-" 不参与序列化但消耗内存），
// 调用方拿到 token + expiresAt + fallbackUsed 三件套足够。
func (c *Client) buildLoginResponse(token string, expiresAt time.Time, bodyBytes []byte, label string, fallbackUsed bool) *types.LoginResponse {
	c.warnSyncCookieToken(token, label)
	return &types.LoginResponse{
		Token:        token,
		ExpiresAt:    expiresAt,
		FallbackUsed: fallbackUsed,
	}
}
