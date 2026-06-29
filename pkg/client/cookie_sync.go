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

	// 优先使用预解析的 baseURLParsed，否则懒解析一次并缓存
	u := c.baseURLParsed
	if u == nil {
		var err error
		u, err = url.Parse(c.baseURL)
		if err != nil {
			return fmt.Errorf("syncCookieToken 失败: 解析 base URL %q 出错: %w", c.baseURL, err)
		}
		c.baseURLParsed = u
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
// 注意：bodyBytes 入参保留签名为后续可能的 RawData 扩展留有余地。
// 当前 RawData 字段仅在测试中使用且 json:"-" 不参与序列化，
// 故直接置 nil 避免对已有 DecodeResponse 的 bodyBytes 二次 JSON 解析。
func (c *Client) buildLoginResponse(token string, expiresAt time.Time, bodyBytes []byte, label string) *types.LoginResponse {
	c.warnSyncCookieToken(token, label)

	return &types.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		RawData:   nil,
	}
}
