package logx

import (
	"regexp"
	"strings"
)

// 敏感 key 集合，大小写不敏感。
var sensitiveKeys = map[string]bool{
	"token":         true,
	"x-auth-token":  true,
	"authorization": true,
	"password":      true,
	"passwd":        true,
	"captcha":       true,
}

func isSensitiveKey(k string) bool {
	return sensitiveKeys[strings.ToLower(strings.TrimSpace(k))]
}

// maskValue 对敏感值做掩码，保留前后 2 字符便于排错。
func maskValue(v string) string {
	if len(v) <= 4 {
		return "***"
	}
	return v[:2] + "***" + v[len(v)-2:]
}

// RedactHeader 对 header 值脱敏，敏感 key 时掩码；Referer 中的 token 查询串也做掩码。
func RedactHeader(k, v string) string {
	if isSensitiveKey(k) {
		return maskValue(v)
	}
	if strings.EqualFold(k, "Referer") && tokenQueryRe.MatchString(v) {
		return tokenQueryRe.ReplaceAllString(v, `${1}***`)
	}
	return v
}

// 匹配 JSON 中敏感 key 的值，大小写不敏感。
var kvRe = regexp.MustCompile(`(?i)"(token|x-auth-token|authorization|password|passwd|captcha)"\s*:\s*"[^"]*"`)

// 匹配 URL 查询串中的 token 参数。
var tokenQueryRe = regexp.MustCompile(`(?i)(token=)[^&\s"]+`)

// RedactBody 对 body 中的敏感 JSON 键值做掩码，对 URL token 查询串也掩码，并截断到 256 字符。
func RedactBody(s string) string {
	red := tokenQueryRe.ReplaceAllString(s, `${1}***`)
	red = kvRe.ReplaceAllStringFunc(red, func(m string) string {
		idx := strings.Index(m, ":")
		if idx < 0 {
			return m
		}
		return m[:idx+1] + `"***"`
	})
	if len(red) > 256 {
		red = red[:256] + "..."
	}
	return red
}

// RedactValue 按 key 判断是否需掩码。
func RedactValue(key, val string) string {
	if isSensitiveKey(key) {
		return maskValue(val)
	}
	return val
}
