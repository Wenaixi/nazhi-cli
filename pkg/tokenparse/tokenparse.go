// Package tokenparse 封装 SSO 登录 token 解析逻辑。
package tokenparse

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultTokenTTL 是 server 不带 expires 信息时的兜底 TTL。
const DefaultTokenTTL = 24 * time.Hour

// ErrLocationParseFailed 是 tokenparse 包的 sentinel 错误。
//
// 调用方（pkg/client/auth.go）拿到本错误后用 fmt.Errorf("%w", client.ErrLocationParseFailed)
// 包装一次，保留 pkg/client 的对外契约。
//
// 为什么不让 tokenparse 直接返回 pkg/client 的 sentinel：
//   - 循环依赖（tokenparse → client → tokenparse）
//   - 测试独立（tokenparse 测试不依赖 client 包状态）
var ErrLocationParseFailed = errors.New("tokenparse: location header parse failed")

// ExtractFromLocation 从 302 Location 头中提取 token 和过期时间。
func ExtractFromLocation(location string) (token string, exp time.Time, err error) {
	u, perr := url.Parse(location)
	if perr != nil {
		return "", time.Time{}, wrapLocationParseErr(perr)
	}
	q := u.Query()
	if t := q.Get("token"); t != "" {
		token = t
	} else if u.Fragment != "" {
		if fToken := extractTokenFromFragment(u.Fragment); fToken != "" {
			token = fToken
		}
	}
	qm := make(map[string]any, len(q))
	for k, vs := range q {
		if len(vs) > 0 {
			qm[k] = vs[0]
		}
	}
	return token, parseExpiresMap(qm), nil
}

// ExtractFromReturnData 从 ReturnData 字节中提取 token 和过期时间。
func ExtractFromReturnData(raw json.RawMessage) (string, time.Time, error) {
	if len(raw) == 0 {
		return "", time.Time{}, errors.New("returnData 为空")
	}
	var data map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return "", time.Time{}, err
	}
	token, ok := data["token"].(string)
	if !ok {
		return "", time.Time{}, errors.New("returnData 中 token 字段类型异常（期望 string）")
	}
	if token == "" {
		return "", time.Time{}, errors.New("returnData 中无 token 字段")
	}
	return token, parseExpiresMap(data), nil
}

// ─── 内部辅助 ───

func parseExpiresMap(q map[string]any) time.Time {
	now := time.Now()
	if v, ok := q["expires_in"]; ok {
		if s, err := valueToString(v); err == nil && s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				return now.Add(time.Duration(n) * time.Second)
			}
		}
	}
	if v, ok := q["exp"]; ok {
		if s, err := valueToString(v); err == nil && s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				return time.Unix(n, 0)
			}
		}
	}
	return now.Add(DefaultTokenTTL)
}

func valueToString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case json.Number:
		return x.String(), nil
	case float64:
		return strconv.FormatInt(int64(x), 10), nil
	default:
		return "", errors.New("valueToString: 不支持的类型")
	}
}

func extractTokenFromFragment(fragment string) string {
	parts := strings.Split(fragment, "&")
	for _, p := range parts {
		if strings.HasPrefix(p, "token=") {
			raw := strings.TrimPrefix(p, "token=")
			decoded, err := url.QueryUnescape(raw)
			if err != nil {
				return raw
			}
			return decoded
		}
	}
	return ""
}

func wrapLocationParseErr(err error) error {
	return errors.Join(ErrLocationParseFailed, err)
}
