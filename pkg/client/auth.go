package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
	"github.com/Wenaixi/nazhi-cli/pkg/tokenparse"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── GetSchoolID ───

// GetSchoolID 根据学号查询学校 ID 和学校名称。
func (c *Client) GetSchoolID(ctx context.Context, username string) (*types.SchoolInfo, error) {
	u := c.ssoURL("/teacher/auth/studentLogin/getSchoolIdByStudentNumber", url.Values{"userName": {username}})

	headers := c.ssoHeaders()
	headers["Referer"] = c.ssoURL("/uiStudentLogin/login", url.Values{"userName": {username}})

	bodyBytes, err := c.httpDo(ctx, http.MethodPost, u, map[string]string{"key": ""}, headers, "application/json")
	if err != nil {
		return nil, fmt.Errorf("GetSchoolID 请求失败: %w", err)
	}

	resp, err := decodeOrInvalidResponse("GetSchoolID", bodyBytes)
	if err != nil {
		return nil, err
	}

	if err := types.CheckCode(resp); err != nil {
		return nil, errors.Join(ErrBusinessRejected, fmt.Errorf("GetSchoolID 业务错误: %w", err))
	}

	// dataList 用 UseNumber 解码：school_id 是标识符而非测量值，必须原样
	// 保留服务端字面量。标准 json.Unmarshal 把数字解为 float64，超过 2^53
	// 的整数会在解码阶段就丢精度（实测 9007199254740993 → 9007199254740992），
	// 任何后续格式化都无法还原。与 tokenparse、file.go 的解码纪律一致。
	schools, err := decodeSchoolListUseNumber(resp.DataList)
	if err != nil {
		return nil, fmt.Errorf("GetSchoolID dataList 解析失败: %w", err)
	}

	if len(schools) == 0 {
		return nil, fmt.Errorf("GetSchoolID: 未找到学校信息")
	}

	school := schools[0]

	// 校验 school_id 为有效数字，防止非数字值被静默传给登录请求。
	//
	// 数值格式化不能直接用 %v：school 来自 map[string]any，JSON 数字解码为
	// float64，而 %v 对 float64 走 %g 语义——7 位及以上整数即被输出为科学
	// 计数法（实测 1234567 → "1.234567e+06"），随后的 ParseInt 必然失败，
	// 合法学校 ID 被误判为「非有效数字」。'f' 格式 + 精度 -1 表示「用最短
	// 表示法精确还原该浮点值」，整数路径输出纯十进制。
	schoolIDRaw, ok := school["school_id"]
	if !ok || schoolIDRaw == nil {
		return nil, fmt.Errorf("%w: GetSchoolID school_id 字段缺失或为 nil", ErrInvalidPayload)
	}
	schoolIDStr := formatSchoolID(schoolIDRaw)
	if _, err := strconv.ParseInt(schoolIDStr, 10, 64); err != nil {
		return nil, fmt.Errorf("%w: GetSchoolID school_id=%q 不是有效数字: %w", ErrInvalidPayload, schoolIDStr, err)
	}
	schoolName := ""
	// 学校名键双兼容——服务端 school_id 用小写键、NAME 用大写键，风格不一致；
	// 部分部署可能返回小写 name。NAME 优先，name 兜底。
	if v, ok := school["NAME"]; ok {
		schoolName = fmt.Sprintf("%v", v)
	} else if v, ok := school["name"]; ok {
		schoolName = fmt.Sprintf("%v", v)
	}

	return &types.SchoolInfo{
		SchoolID:   schoolIDStr,
		SchoolName: schoolName,
	}, nil
}

// formatSchoolID 把 school_id 规范化为纯十进制字符串。
//
// 服务端返回 JSON 数字时解码为 float64，直接 %v 会走 %g 语义输出科学
// 计数法（实测 7 位整数 1234567 → "1.234567e+06"），导致随后的 ParseInt
// 失败。这里对数值型走 strconv.FormatFloat(f, 'f', -1, 64)——'f' 表示不用
// 指数，精度 -1 表示用能精确还原该浮点值的最短表示，整数即输出纯十进制。
//
// json.Number（UseNumber 解码路径）直接取其字符串，保留服务端原始字面量，
// 避免大整数经 float64 往返丢失精度。string 与其他类型保持原样，交由
// 调用方的 ParseInt 裁决。
func formatSchoolID(v any) string {
	switch n := v.(type) {
	case float64:
		// 'f' 不用指数，精度 -1 用最短精确表示——整数即输出纯十进制。
		// %v 会走 %g 语义，把 7 位以上整数输出成科学计数法。
		return strconv.FormatFloat(n, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(n), 'f', -1, 32)
	case json.Number:
		return n.String()
	case int:
		return strconv.Itoa(n)
	case int64:
		return strconv.FormatInt(n, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// decodeSchoolListUseNumber 解码 getSchoolIdByStudentNumber 的 dataList，
// 数字保留为 json.Number 而非 float64。
//
// school_id 是标识符：标准解码会把它变成 float64，超过 2^53 的整数在解码
// 阶段即丢精度（实测 9007199254740993 → 9007199254740992），任何后续格式化
// 都无法还原。UseNumber 让字面量原样到达 formatSchoolID。
// 纪律与 tokenparse.ExtractFromReturnData、file.go 的 returnData 解码一致。
func decodeSchoolListUseNumber(raw *json.RawMessage) ([]map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(*raw))
	dec.UseNumber()
	var rows []map[string]any
	if err := dec.Decode(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ─── Login ───

// md5Hex 计算小写十六进制 MD5（与官方五育 APK hex_md5 一致）。
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// Login 完成五育活动端免验证码登录并返回 Token。
//
// 登录流程：
//   - 若请求未携带 SchoolID，先通过匿名接口 GetSchoolID 自动推断；
//   - 密码本地计算 MD5（hex_md5 口径，与官方 APK 一致）后，
//     POST /uiActivityLogin/studentLogin 免验证码直签 JWT；
//   - 解析 returnData.token 并同步到 cookie jar。
//
// 该端点无验证码、无 ticket；与主站 X-Auth-Token 同认证体系。
func (c *Client) Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error) {
	if c.http == nil {
		return nil, fmt.Errorf("Login 失败: HTTP 客户端为 nil，无法发送请求")
	}

	// 步骤 1: 未指定 SchoolID 时自动推断（匿名接口，无验证码前置）
	schoolID := req.SchoolID
	if schoolID == "" {
		info, err := c.GetSchoolID(ctx, req.Username)
		if err != nil {
			return nil, fmt.Errorf("Login GetSchoolID 失败: %w", err)
		}
		schoolID = info.SchoolID
	}

	loginBody := map[string]string{
		"schoolId": schoolID,
		"username": req.Username,
		"password": md5Hex(req.Password),
	}

	httpResp, err := c.rawDoWithResp(ctx, http.MethodPost,
		c.ssoURL("/uiActivityLogin/studentLogin", nil),
		loginBody, c.ssoHeaders(), "",
	)
	if err != nil {
		return nil, fmt.Errorf("Login 请求失败: %w", err)
	}
	defer drainAndClose(httpResp.Body)

	// 契约：Login validate 端点响应体同样封顶 1MB。
	// 与 request.go doBizGet/httpDo 同构——防异常/被劫持 SSO 塞超大 body 造成内存放大。
	// 302 分支不读 body（只取 Location 头），仅 200 与其它状态码分支受影响。
	bodyBytes, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("Login 读取响应体失败: status=%d read=%d bytes: %w",
			httpResp.StatusCode, len(bodyBytes), err)
	}
	if len(bodyBytes) > maxResponseBodySize {
		return nil, fmt.Errorf("%w: Login 响应体超过 %d 字节上限", ErrLoginRejected, maxResponseBodySize)
	}
	bodySnippet := redactSnippet(bodyBytes, 100)

	if httpResp.StatusCode == http.StatusOK {
		loginResp, err := types.DecodeResponse(bodyBytes)
		if err != nil {
			c.logDebugCtx(ctx, "Login 200 响应 body 解析失败: %v body=%s", err, bodySnippet)
			return nil, fmt.Errorf("%w: 响应 body JSON 解析失败: %w", ErrLoginRejected, err)
		}
		if err := types.CheckCode(loginResp); err != nil {
			return nil, fmt.Errorf("登录失败: %w", errors.Join(ErrLoginRejected, err))
		}
		if loginResp.ReturnData == nil || bytes.Equal(bytes.TrimSpace(*loginResp.ReturnData), []byte("null")) {
			c.logDebugCtx(ctx, "Login 200 响应 returnData 为 null body=%s", bodySnippet)
			return nil, fmt.Errorf("%w: returnData 为 null", ErrLoginRejected)
		}
		token, expiresAt, err := tokenparse.ExtractFromReturnData(*loginResp.ReturnData)
		if err != nil {
			c.logDebugCtx(ctx, "Login 200 响应 extractToken 失败: %v body=%s", err, bodySnippet)
			return nil, fmt.Errorf("%w: 200 响应中未找到 token: %w", ErrLoginRejected, err)
		}
		c.warnIfExpiresAtFallback(expiresAt, "200")
		if err := c.syncCookieToken(token); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCookieSyncFailed, err)
		}
		return &types.LoginResponse{Token: token, ExpiresAt: expiresAt}, nil
	}

	if httpResp.StatusCode == http.StatusFound {
		location := httpResp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("%w: 302 响应中未找到 Location 头", ErrLoginRejected)
		}
		token, expiresAt, locErr := tokenparse.ExtractFromLocation(location)
		if locErr != nil {
			c.logDebugCtx(ctx, "Login 302: Location 头解析失败: %v location=%s", locErr, location)
			return nil, fmt.Errorf("%w: Location 头解析失败: %w", ErrLoginRejected, locErr)
		}
		if token == "" {
			return nil, fmt.Errorf("%w: Location 头中未找到 token: %s", ErrLoginRejected, logx.RedactBody(location))
		}
		c.warnIfExpiresAtFallback(expiresAt, "302 fallback")
		if err := c.syncCookieToken(token); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrCookieSyncFailed, err)
		}
		return &types.LoginResponse{Token: token, ExpiresAt: expiresAt}, nil
	}

	// 非 200/302 状态码先按 classifyHTTPStatus 分类，
	// 429→ErrRateLimited、5xx→ErrServiceUnavailable、其余→ErrLoginRejected。
	// 修复前一律包 ErrLoginRejected，CLI 把登录被限流/服务端故障误报为
	// 「凭证错误」exit 1、不退避；errors.Is(err, ErrRateLimited) 现在可精确识别限流。
	sentinel := classifyHTTPStatus(httpResp.StatusCode, ErrLoginRejected)

	errResp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		c.logDebugCtx(ctx, "Login 非预期状态码 %d 响应非 JSON: %v body=%s", httpResp.StatusCode, err, bodySnippet)
	} else if err := types.CheckCode(errResp); err != nil {
		return nil, fmt.Errorf("%w: code=%d msg=%s", sentinel, errResp.Code, types.DerefOr(errResp.Msg, "登录失败"))
	}
	// 错误消息附带 redactSnippet 截断脱敏摘要：非预期状态码的典型场景是 nginx 503、
	// CDN challenge 等 HTML 响应；不带 body 片段时用户难以定位根因。
	// 摘要经 redactSnippet 先粗截再脱敏，与 request.go 同类分支脱敏口径拉平。
	return nil, fmt.Errorf("%w: 非预期状态码 %d body=%s",
		sentinel, httpResp.StatusCode, redactSnippet(bodyBytes, 100))
}

// warnIfExpiresAtFallback 在 expiresAt 异常时输出 WARN 日志。两条 Login 路径
// （200/302）共用，避免重复。
//
// 检测两类异常:
//
//  1. fallback 触发：server 响应没带 expires_in/exp 且 JWT payload 也无 exp 声明，
//     退回到 now+24h 兜底。此时 remaining ≈24h（22h–25h 区间即视为兜底）。
//  2. 已过期/即将过期：剩余寿命 < expiresFallbackThreshold，server 给的 exp
//     已是过去时间（或剩余过短），首次业务调用会立即 401。
//
// tokenparse 的 extractExpFromJWT 从 JWT payload 提取 exp 声明后，
// server 不传 expires_in/exp 时不再立即触发 24h 兜底 warn——JWT 自身的 exp 声明
// 仍是服务端签发的合法过期时间。仅当检测到 24h 兜底（22h–25h 区间）时才 warn。
// 两类异常均覆盖：过去时间的 remaining 为负，同样落入检测范围。
func (c *Client) warnIfExpiresAtFallback(expiresAt time.Time, label string) {
	if c.logger == nil {
		return
	}
	remaining := time.Until(expiresAt)
	// 检测 24h 兜底：remaining ≈24h（22h–25h 区间即视为兜底）。
	// JWT 自身的 exp（如 14 天）不是 fallback，只有落在该区间才是真兜底。
	fallback := 1 * time.Hour
	if remaining > tokenparse.DefaultTokenTTL-2*fallback &&
		remaining < tokenparse.DefaultTokenTTL+fallback {
		c.logger.Warn("Login token 剩余寿命恰好 ≈24h，服务器可能未带 expires_in/exp",
			"label", label,
			"remaining", remaining.Round(time.Second),
			"expiresAt", expiresAt.Format(time.RFC3339))
		return
	}
	if remaining < fallback {
		c.logger.Warn("Login token 已过期或剩余 < 1h，首次业务调用将立即 401",
			"label", label,
			"remaining", remaining.Round(time.Second),
			"expiresAt", expiresAt.Format(time.RFC3339))
	}
}

// ssoURL 拼接 SSO 域名的完整 URL。
// 与 bizURL helper 对称，统一管理 SSO URL 拼接。
func (c *Client) ssoURL(path string, q url.Values) string {
	if len(q) > 0 {
		return c.ssoBaseURL + path + "?" + q.Encode()
	}
	return c.ssoBaseURL + path
}
