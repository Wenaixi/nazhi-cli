package client

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

// syncCookieToken 将 JWT token 同步到 HTTP cookie jar 中（X-Auth-Token）。
// 业务 API 通过 cookie 鉴权而非 Authorization 头。
//
// baseURL 在 New() 阶段已预解析到 c.baseURLParsed，调用时无需重复 url.Parse。
// 若直接构造 Client（绕过 New()），则懒解析一次并缓存回 c.baseURLParsed。
//
// c.baseURLParsed 为 atomic.Pointer[url.URL]，并发访问全部原子化。
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

	// cookiejar.SetCookies 对非 http/https 的 scheme 直接静默 return（标准库
	// 行为，无返回值），此时 cookie 不会写入 jar，但调用方若收到 nil 会误以为
	// 同步成功——后果是所有业务接口返回空 dataList，且排查方向被误导到「服务端
	// 返回空」而非「配置侧 cookie 未落地」。这正是 ErrCookieSyncFailed 存在的
	// 同类症状，故在写入前显式拦截。
	switch u.Scheme {
	case "http", "https":
		// 可写入，继续。
	case "":
		return fmt.Errorf("%w: base URL %q 缺少 scheme（应为 http:// 或 https:// 开头），cookie 无法写入",
			ErrCookieSyncFailed, c.baseURL)
	default:
		return fmt.Errorf("%w: base URL %q 的 scheme %q 不受支持（cookiejar 仅支持 http/https）",
			ErrCookieSyncFailed, c.baseURL, u.Scheme)
	}

	jar.SetCookies(u, []*http.Cookie{{
		Name:  "X-Auth-Token",
		Value: token,
		Path:  "/",
	}})
	c.logDebug("X-Auth-Token 已同步到 cookie jar（%s）", c.baseURL)
	return nil
}
