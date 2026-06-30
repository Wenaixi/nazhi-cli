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
func (c *Client) buildLoginResponse(token string, expiresAt time.Time, bodyBytes []byte, label string) *types.LoginResponse {
	c.warnSyncCookieToken(token, label)

	// 用 json.Unmarshal 解析原始 body 为泛型 map，供 RawData 字段使用。
	//
	// F2 修复：partial decode 防御。json.Decoder 默认读完一个 JSON value 后，
	// 后跟非空白字符时返回 err=nil（默认 Mode=non_strict），导致 rawData 已
	// 填但调用方不知道后面还有未解析内容——下游用户拿到"看起来有效"的
	// 半成品 map 而不知其不完整。
	//
	// 新行为：
	//   1. 解析失败（err != nil）：完全兜底空 map（向后兼容）
	//   2. 解析成功后用 dec.More() 检查 reader 残留——
	//      有残留即视为 partial（log warn + RawData = nil 防止下游误用）
	var rawData map[string]any
	if len(bodyBytes) > 0 {
		dec := json.NewDecoder(bytes.NewReader(bodyBytes))
		dec.UseNumber()
		if err := dec.Decode(&rawData); err != nil {
			rawData = make(map[string]any)
		} else if dec.More() {
			// json.Decoder 已读完第一个 JSON value，reader 还有内容未解析
			// → 原行为 silent 成功，下游拿到半成品。新行为：log warn + 清零。
			if c.logger != nil {
				c.logger.Warn("buildLoginResponse: RawData partial decode 失败",
					"keys", len(rawData),
					"tip", "body 含多个 JSON value 或尾部残留")
			}
			rawData = nil
		}
	}
	return &types.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		RawData:   rawData,
	}
}
