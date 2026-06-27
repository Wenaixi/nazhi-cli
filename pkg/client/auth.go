package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── InitSession ───

// InitSession 访问登录页建立 JSESSIONID Cookie。
// 内部流程中自动调用，一般不需要外部显式调用。
func (c *Client) InitSession(ctx context.Context) error {
	u := c.ssoURL("/uiStudentLogin/login")
	if _, err := c.doBizGet(ctx, u, c.ssoHeaders()); err != nil {
		return fmt.Errorf("InitSession 失败: %w", err)
	}
	return nil
}

// ─── GetSchoolID ───

// GetSchoolID 根据学号查询学校 ID 和学校名称。
func (c *Client) GetSchoolID(ctx context.Context, username string) (schoolID string, schoolName string, err error) {
	u := c.ssoURL("/teacher/auth/studentLogin/getSchoolIdByStudentNumber?" + url.Values{"userName": {username}}.Encode())

	headers := c.ssoHeaders()
	headers["Referer"] = c.ssoURL("/uiStudentLogin/login?" + url.Values{"userName": {username}}.Encode())

	bodyBytes, err := c.doRequest(ctx, http.MethodPost, u, map[string]string{"key": ""}, headers, "application/json")
	if err != nil {
		return "", "", fmt.Errorf("GetSchoolID 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return "", "", fmt.Errorf("GetSchoolID 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		return "", "", fmt.Errorf("GetSchoolID 业务错误: %w", errors.Join(ErrBusinessRejected, err))
	}

	schools, err := types.DecodeDataList[map[string]any](resp)
	if err != nil {
		return "", "", fmt.Errorf("GetSchoolID dataList 解析失败: %w", err)
	}

	if len(schools) == 0 {
		return "", "", fmt.Errorf("GetSchoolID: 未找到学校信息")
	}

	school := schools[0]
	schoolID = fmt.Sprintf("%v", school["school_id"])
	if v, ok := school["NAME"]; ok {
		schoolName = fmt.Sprintf("%v", v)
	}

	return schoolID, schoolName, nil
}

// ─── Login ───

const (
	maxOCRAttemptsPerImage = 1
	maxOCRImagesTotal      = 99
	ocrTimeout             = 30 * time.Second
)

// Login 完成 SSO 登录并返回 Token。
func (c *Client) Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error) {
	if c.ocr == nil {
		return nil, ErrOCRNotConfigured
	}

	if err := c.InitSession(ctx); err != nil {
		return nil, fmt.Errorf("Login InitSession 失败: %w", err)
	}

	schoolID := req.SchoolID
	if schoolID == "" {
		var err error
		schoolID, _, err = c.GetSchoolID(ctx, req.Username)
		if err != nil {
			return nil, fmt.Errorf("Login GetSchoolID 失败: %w", err)
		}
	}

	captcha, err := c.ocrRecognizeWithRetry(ctx)
	if err != nil {
		return nil, fmt.Errorf("Login OCR 自动识别验证码失败: %w", err)
	}
	c.logDebug("OCR 识别结果: %s", captcha)

	if err := c.validateCaptcha(ctx, captcha); err != nil {
		return nil, fmt.Errorf("Login 验证码预校验未通过: %w", err)
	}

	loginBody := map[string]string{
		"schoolId": schoolID,
		"username": req.Username,
		"password": req.Password,
	}

	httpResp, err := c.doRequestWithResp(ctx, http.MethodPost,
		c.ssoURL("/teacher/auth/studentLogin/validate"),
		loginBody, c.ssoHeaders(), "",
	)
	if err != nil {
		return nil, fmt.Errorf("Login 请求失败: %w", err)
	}
	defer drainAndClose(httpResp.Body)

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("Login 读取响应体失败: status=%d read=%d bytes: %w",
			httpResp.StatusCode, len(bodyBytes), err)
	}

	if httpResp.StatusCode == http.StatusOK {
		var loginResp types.UnifiedResponse
		if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
			c.logDebug("Login 200 响应 body 解析失败: %v body=%s", err, string(bodyBytes))
		} else {
			if loginResp.Code != 1 {
				return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, loginResp.Code, derefOr(loginResp.Msg, "登录失败"))
			}
			token, expiresAt, err := extractTokenFromReturnData(loginResp)
			if err != nil {
				c.logDebug("Login 200 响应 extractToken 失败: %v body=%s", err, string(bodyBytes))
				return nil, fmt.Errorf("%w: 200 响应中未找到 token: %v", ErrLoginRejected, err)
			}
			if time.Until(expiresAt) > defaultTokenTTL-expiresFallbackThreshold {
				c.logger.Warn("Login 200: returnData 未带 expires_in/exp，使用 now+24h 兜底")
			}
			return c.buildLoginResponse(token, expiresAt, bodyBytes, "200"), nil
		}
		return nil, fmt.Errorf("%w: 200 响应中未找到 token", ErrLoginRejected)
	}

	if httpResp.StatusCode == http.StatusFound {
		location := httpResp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("%w: 302 响应中未找到 Location 头", ErrLoginRejected)
		}
		token, expiresAt, locErr := extractTokenFromLocation(location)
		if locErr != nil {
			c.logDebug("Login 302: Location 头解析失败: %v location=%s", locErr, location)
			return nil, fmt.Errorf("%w: Location 头解析失败", ErrLoginRejected)
		}
		if token == "" {
			return nil, fmt.Errorf("%w: Location 头中未找到 token: %s", ErrLoginRejected, location)
		}
		if time.Until(expiresAt) > defaultTokenTTL-expiresFallbackThreshold {
			c.logger.Warn("Login 302 fallback: Location 未带 expires_in/exp，使用 now+24h 兜底")
		}
		return c.buildLoginResponse(token, expiresAt, bodyBytes, "302 fallback"), nil
	}

	var errResp types.UnifiedResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		c.logDebug("Login 非预期状态码 %d 响应非 JSON: %v body=%s", httpResp.StatusCode, err, string(bodyBytes))
	} else if errResp.Code != 1 {
		return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, errResp.Code, derefOr(errResp.Msg, "登录失败"))
	}
	return nil, fmt.Errorf("%w: 非预期状态码 %d", ErrLoginRejected, httpResp.StatusCode)
}

// ─── 验证码内部辅助 ───

func (c *Client) validateCaptcha(ctx context.Context, captcha string) error {
	bodyBytes, err := c.doRequest(ctx, http.MethodPost,
		c.ssoURL("/uiStudentLogin/validateCaptcha"),
		map[string]string{"captcha": captcha},
		c.ssoHeaders(), "",
	)
	if err != nil {
		return fmt.Errorf("验证码预校验请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return fmt.Errorf("验证码预校验响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		return fmt.Errorf("验证码校验失败: %w", errors.Join(ErrBusinessRejected, err))
	}

	return nil
}

// ocrRecognizeWithRetry 多图多试策略识别验证码。
func (c *Client) ocrRecognizeWithRetry(ctx context.Context) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ocrTimeout)
		defer cancel()
	}
	var lastErr error
	for imgIdx := 0; imgIdx < maxOCRImagesTotal; imgIdx++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			c.logDebug("OCR 循环顶部检测到 ctx cancel（img=%d）: %v", imgIdx+1, ctxErr)
			return "", fmt.Errorf("OCR 识别被 ctx cancel（已重试 %d 次）: %w", imgIdx, ctxErr)
		}
		imgBytes, err := c.fetchCaptchaImage(ctx)
		if err != nil {
			lastErr = err
			c.logDebug("OCR 获取第 %d 张验证码失败: %v", imgIdx+1, err)
			continue
		}
		text, err := c.safeOCRRecognize(imgBytes)
		if err != nil {
			lastErr = err
			c.logDebug("OCR 第 %d 张图失败: %v", imgIdx+1, err)
		} else if text == "" {
			lastErr = fmt.Errorf("空白结果")
			c.logDebug("OCR 第 %d 张图结果为空白", imgIdx+1)
		} else {
			c.logDebug("OCR 识别成功: img=%d result=%s", imgIdx+1, text)
			return text, nil
		}
	}
	return "", fmt.Errorf("OCR 识别 %d 张图 × %d 次（共 %d 次）均失败，最后错误: %w",
		maxOCRImagesTotal, maxOCRAttemptsPerImage,
		maxOCRImagesTotal*maxOCRAttemptsPerImage, lastErr)
}

var captchaSeq atomic.Int64

// fetchCaptchaImage 拉取一张新的验证码图片。
//
// F1: 删除冗余的 t= 时间戳参数（seq 原子计数器已足够防缓存碰撞），
// 改用 url.Values 编码替代 fmt.Sprintf+strconv.FormatInt 混合拼接风格。
func (c *Client) fetchCaptchaImage(ctx context.Context) ([]byte, error) {
	seq := captchaSeq.Add(1)
	u := c.ssoURL("/kaptcha/kaptcha.jpg?" + url.Values{"seq": {strconv.FormatInt(seq, 10)}}.Encode())
	imgBytes, err := c.doBizGet(ctx, u, c.ssoHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取验证码图片失败: %w", err)
	}
	if len(imgBytes) == 0 {
		return nil, fmt.Errorf("获取验证码图片响应为空 status=200")
	}
	return imgBytes, nil
}

// ─── 内部辅助 ───

// extractTokenFromLocation 从 302 Location 头中提取 token 和过期时间。
//
// F2: 缓存 u.Query() 结果 q，消除重复解析（之前 token 提取和 query 转换各调一次）。
// F3: 内联 queryToMap（已删除），直接转换 url.Values → map[string]any。
func extractTokenFromLocation(location string) (string, time.Time, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrLocationParseFailed, err)
	}
	q := u.Query()
	var token string
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

const defaultTokenTTL = 24 * time.Hour

const expiresFallbackThreshold = 1 * time.Hour

// parseExpiresMap 是 302 Location query 与 200 returnData 共用的过期时间解析器。
//
// F4: 删除 parseReturnDataExpires 包装函数（与 parseExpiresMap 相同），
//
//	调用方 extractTokenFromReturnData 直接调 parseExpiresMap(data)。
//
// F5: 不再引用已删除的 parseLocationExpires。
// F6: 函数开头调一次 time.Now() 引用为 now 变量，替代三处重复的 time.Now() 调用。
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
	return now.Add(defaultTokenTTL)
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
		return fmt.Sprintf("%v", v), nil
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

// extractTokenFromReturnData 尝试从统一响应的 returnData 中提取 token。
//
// F4: 直接调 parseExpiresMap(data) 而非 parseReturnDataExpires（已删除）。
func extractTokenFromReturnData(resp types.UnifiedResponse) (string, time.Time, error) {
	if resp.ReturnData == nil {
		return "", time.Time{}, fmt.Errorf("returnData 为空")
	}
	var data map[string]any
	dec := json.NewDecoder(bytes.NewReader(*resp.ReturnData))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return "", time.Time{}, err
	}
	token, ok := data["token"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("returnData 中 token 字段类型异常（期望 string）")
	}
	if token == "" {
		return "", time.Time{}, fmt.Errorf("returnData 中无 token 字段")
	}
	return token, parseExpiresMap(data), nil
}

func parseRawData(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func derefOr(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

func (c *Client) syncCookieToken(token string) error {
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

func (c *Client) warnSyncCookieToken(token, label string) {
	if err := c.syncCookieToken(token); err != nil {
		c.logger.Warn("Login "+label+" 后同步 token 到 cookie 失败", "err", err.Error())
	}
}

func (c *Client) buildLoginResponse(token string, expiresAt time.Time, bodyBytes []byte, label string) *types.LoginResponse {
	c.warnSyncCookieToken(token, label)
	return &types.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		RawData:   parseRawData(bodyBytes),
	}
}
