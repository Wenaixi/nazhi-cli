package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// syncCookieToken 将 JWT token 同步到 HTTP cookie jar 中（X-Auth-Token）。
// 业务 API 通过 cookie 鉴权而非 Authorization 头。
func (c *Client) syncCookieToken(token string) error {
	if c.http == nil {
		return fmt.Errorf("syncCookieToken: HTTP client 为 nil，无法同步 token 到 cookie")
	}
	jar, ok := c.http.Jar.(*cookiejar.Jar)
	if !ok {
		return fmt.Errorf("syncCookieToken: HTTP client 的 Jar 不是 *cookiejar.Jar（实际类型 %T），X-Auth-Token 无法同步到 cookie。"+
			"修复：用 client.New() 默认 HTTP 客户端，或显式 &http.Client{Jar: cookiejar.New(nil)} 创建",
			c.http.Jar)
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("syncCookieToken: 解析 base URL %q 失败: %w", c.baseURL, err)
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
func (c *Client) buildLoginResponse(token string, expiresAt time.Time, bodyBytes []byte, label string) *types.LoginResponse {
	c.warnSyncCookieToken(token, label)

	// 用 json.Unmarshal 解析原始 body 为泛型 map，供 RawData 字段使用
	var rawData map[string]any
	if len(bodyBytes) > 0 {
		dec := json.NewDecoder(bytes.NewReader(bodyBytes))
		dec.UseNumber()
		_ = dec.Decode(&rawData) // 解析失败返回 nil
	}
	return &types.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		RawData:   rawData,
	}
}
