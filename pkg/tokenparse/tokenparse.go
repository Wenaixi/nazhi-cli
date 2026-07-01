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

// ─── 哨兵错误 ───

// ErrTokenReturnDataEmpty 表示 returnData 为空。
var ErrTokenReturnDataEmpty = errors.New("returnData 为空")

// ErrTokenTypeMismatch 表示 returnData 中 token 字段类型异常（期望 string）。
var ErrTokenTypeMismatch = errors.New("returnData 中 token 字段类型异常（期望 string）")

// ErrTokenFieldMissing 表示 returnData 中无 token 字段或 token 为空。
var ErrTokenFieldMissing = errors.New("returnData 中无 token 字段")

// DefaultTokenTTL 是 server 不带 expires 信息时的兜底 TTL。
const DefaultTokenTTL = 24 * time.Hour

// ExtractFromLocation 从 302 Location 头中提取 token 和过期时间。
//
// 错误处理：url.Parse 失败时直接返回底层错误（net/url 已是可读的 parse error）。
// 不再定义包级 ErrLocationParseFailed sentinel——历史版本曾导出过该 sentinel，
// 但 auth.go 包装时未用 %w 链入，调用方 errors.Is 永远不命中，纯死代码。
// （dead-code 重构：refactor/remove-dead-location-parse-sentinel）
func ExtractFromLocation(location string) (token string, exp time.Time, err error) {
	u, perr := url.Parse(location)
	if perr != nil {
		return "", time.Time{}, perr
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
		return "", time.Time{}, ErrTokenReturnDataEmpty
	}
	var data map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return "", time.Time{}, err
	}
	token, ok := data["token"].(string)
	if !ok {
		return "", time.Time{}, ErrTokenTypeMismatch
	}
	if token == "" {
		return "", time.Time{}, ErrTokenFieldMissing
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
		return strconv.FormatFloat(x, 'f', 0, 64), nil
	default:
		return "", errors.New("valueToString: 不支持的类型")
	}
}

func extractTokenFromFragment(fragment string) string {
	// 注意：url.Parse 已将 fragment 百分比解码（u.Fragment 是解码后的值），
	// 所以不再做二次 URL 解码。直接 TrimPrefix 提取 token 值。
	parts := strings.Split(fragment, "&")
	for _, p := range parts {
		if strings.HasPrefix(p, "token=") {
			return strings.TrimPrefix(p, "token=")
		}
	}
	return ""
}
