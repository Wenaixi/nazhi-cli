package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── InitSession ───

// InitSession 访问登录页建立 JSESSIONID Cookie。
// 内部流程中自动调用，一般不需要外部显式调用。
func (c *Client) InitSession(ctx context.Context) error {
	url := c.ssoURL("/uiStudentLogin/login")
	resp, err := c.doRequestWithResp(ctx, http.MethodGet, url, nil, c.ssoHeaders(), "")
	if err != nil {
		return fmt.Errorf("InitSession 失败: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("InitSession 返回非 200: %d", resp.StatusCode)
	}
	return nil
}

// ─── GetSchoolID ───

// GetSchoolID 根据学号查询学校 ID 和学校名称。
func (c *Client) GetSchoolID(ctx context.Context, username string) (schoolID string, schoolName string, err error) {
	url := c.ssoURL("/teacher/auth/studentLogin/getSchoolIdByStudentNumber?userName=" + username)

	headers := c.ssoHeaders()
	headers["Referer"] = c.ssoBaseURL + "/uiStudentLogin/login?userName=" + username

	bodyBytes, err := c.doRequest(ctx, http.MethodPost, url, map[string]string{"key": ""}, headers, "application/json")
	if err != nil {
		return "", "", fmt.Errorf("GetSchoolID 请求失败: %w", err)
	}

	resp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		return "", "", fmt.Errorf("GetSchoolID 响应解析失败: %w", err)
	}

	if err := types.CheckCode(resp); err != nil {
		return "", "", fmt.Errorf("GetSchoolID 业务错误: %w", err)
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
	} else if v, ok := school["school_name"]; ok {
		schoolName = fmt.Sprintf("%v", v)
	}

	return schoolID, schoolName, nil
}

// ─── Login ───

// Login 完成 SSO 登录并返回 Token。
// 内部流程：InitSession → GetSchoolID → 获取验证码图片 → OCR 识别（最多 99 次重试同一张图片）→ 预校验 → 正式登录
func (c *Client) Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error) {
	// 1. 建立 session
	if err := c.InitSession(ctx); err != nil {
		return nil, fmt.Errorf("Login InitSession 失败: %w", err)
	}

	// 2. 获取学校 ID（如果未提供）
	schoolID := req.SchoolID
	if schoolID == "" {
		var err error
		schoolID, _, err = c.GetSchoolID(ctx, req.Username)
		if err != nil {
			return nil, fmt.Errorf("Login GetSchoolID 失败: %w", err)
		}
	}

	// 3. OCR 自动识别验证码
	const maxOCRRetries = 99
	captcha, err := c.ocrRecognizeWithRetry(ctx, maxOCRRetries)
	if err != nil {
		return nil, fmt.Errorf("Login OCR 自动识别验证码失败（已重试 %d 次）: %w", maxOCRRetries, err)
	}
	c.logDebug("OCR 识别结果: %s", captcha)

	// 预校验验证码
	if err := c.validateCaptcha(ctx, captcha); err != nil {
		return nil, fmt.Errorf("Login 验证码预校验未通过: %w", err)
	}

	// 4. POST 登录（HAR 验证：请求体无 captcha 字段，captcha 已由 validateCaptcha 单独完成）
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
	defer httpResp.Body.Close()

	bodyBytes, _ := io.ReadAll(httpResp.Body)

	// 5. 优先解析 200 JSON 响应（HAR 验证：登录响应 HTTP 200，body 含 returnData.token）
	if httpResp.StatusCode == http.StatusOK {
		var loginResp types.UnifiedResponse
		if json.Unmarshal(bodyBytes, &loginResp) == nil && loginResp.Code == 1 {
			token, expiresAt, err := extractTokenFromReturnData(loginResp)
			if err == nil {
				// Cookie 同步：将 X-Auth-Token 写入 cookie jar，供后续业务请求使用
				c.syncCookieToken(token)
				return &types.LoginResponse{
					Token:     token,
					ExpiresAt: expiresAt,
					RawData:   parseRawData(bodyBytes),
				}, nil
			}
		}
		return nil, fmt.Errorf("%w: 200 响应中未找到 token", ErrLoginRejected)
	}

	// 6. Fallback：302 Location 头提取 token
	if httpResp.StatusCode == http.StatusFound {
		location := httpResp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("%w: 302 响应中未找到 Location 头", ErrLoginRejected)
		}
		token := extractTokenFromLocation(location)
		if token == "" {
			return nil, fmt.Errorf("%w: Location 头中未找到 token: %s", ErrLoginRejected, location)
		}
		// Cookie 同步
		c.syncCookieToken(token)
		return &types.LoginResponse{
			Token:     token,
			ExpiresAt: time.Now().Add(24 * time.Hour),
			RawData:   parseRawData(bodyBytes),
		}, nil
	}

	// 非预期状态码
	var errResp types.UnifiedResponse
	if json.Unmarshal(bodyBytes, &errResp) == nil {
		if errResp.Code != 1 {
			return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, errResp.Code, stringPtrOr(errResp.Msg, "登录失败"))
		}
	}
	return nil, fmt.Errorf("%w: 非预期状态码 %d", ErrLoginRejected, httpResp.StatusCode)
}

// ─── 验证码内部辅助 ───

// validateCaptcha 预校验验证码。
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
		return fmt.Errorf("验证码校验失败: %w", err)
	}

	return nil
}

// ocrRecognizeWithRetry 获取一次验证码图片，然后对同一张图片重复 OCR 识别最多 retries 次。
func (c *Client) ocrRecognizeWithRetry(ctx context.Context, retries int) (string, error) {
	// 只获取一次验证码图片
	url := c.ssoURL("/kaptcha/kaptcha.jpg?t=" + fmt.Sprintf("%d", time.Now().UnixMilli()))
	resp, err := c.doRequestWithResp(ctx, http.MethodGet, url, nil, c.ssoHeaders(), "")
	if err != nil {
		return "", fmt.Errorf("获取验证码图片失败: %w", err)
	}
	defer resp.Body.Close()

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取验证码图片失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK || len(imgBytes) == 0 {
		return "", fmt.Errorf("获取验证码图片响应异常 status=%d len=%d", resp.StatusCode, len(imgBytes))
	}

	// 对同一张图片多次 OCR 识别
	var lastErr error
	for i := range retries {
		captchaText, err := c.ocr.Recognize(imgBytes)
		if err != nil {
			lastErr = err
			c.logDebug("OCR 第 %d 次识别失败: %v", i+1, err)
			continue
		}
		if captchaText == "" {
			lastErr = fmt.Errorf("空白结果")
			c.logDebug("OCR 第 %d 次结果为空白", i+1)
			continue
		}
		return captchaText, nil
	}

	return "", fmt.Errorf("OCR 识别重试 %d 次均失败，最后错误: %w", retries, lastErr)
}

// ─── 内部辅助 ───

// extractTokenFromLocation 从 302 Location 头中提取 token 查询参数。
// 使用 net/url 解析，正确处理 URL encoding、fragment、复杂 query。
func extractTokenFromLocation(location string) string {
	u, err := url.Parse(location)
	if err != nil {
		return ""
	}
	// 优先 query 参数
	if token := u.Query().Get("token"); token != "" {
		return token
	}
	// 兜底：fragment 中也可能有 token（HAR 验证发现个别场景）
	if u.Fragment != "" {
		if fToken := extractTokenFromFragment(u.Fragment); fToken != "" {
			return fToken
		}
	}
	return ""
}

// extractTokenFromFragment 从 fragment 字符串中提取 token。
func extractTokenFromFragment(fragment string) string {
	parts := strings.Split(fragment, "&")
	for _, p := range parts {
		if strings.HasPrefix(p, "token=") {
			return strings.TrimPrefix(p, "token=")
		}
	}
	return ""
}

// extractTokenFromReturnData 尝试从统一响应的 returnData 中提取 token。
func extractTokenFromReturnData(resp types.UnifiedResponse) (string, time.Time, error) {
	if resp.ReturnData == nil {
		return "", time.Time{}, fmt.Errorf("returnData 为空")
	}
	var data map[string]any
	if err := json.Unmarshal(*resp.ReturnData, &data); err != nil {
		return "", time.Time{}, err
	}
	token, _ := data["token"].(string)
	if token == "" {
		return "", time.Time{}, fmt.Errorf("returnData 中无 token 字段")
	}
	return token, time.Time{}, nil
}

// parseRawData 将原始 JSON 字节解析为 map 用于保留完整数据。
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

// stringPtrOr 返回字符串指针的值，如果为 nil 则返回默认值。
func stringPtrOr(s *string, def string) string {
	if s == nil {
		return def
	}
	return *s
}

// syncCookieToken 将 X-Auth-Token 同步写入 cookie jar，
// 使其在业务 API 请求中自动携带（参考 v1 session.cookies.set 模式）。
func (c *Client) syncCookieToken(token string) {
	jar, ok := c.http.Jar.(*cookiejar.Jar)
	if !ok {
		return
	}
	for _, raw := range []string{c.ssoBaseURL, c.baseURL} {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		jar.SetCookies(u, []*http.Cookie{{
			Name:  "X-Auth-Token",
			Value: token,
			Path:  "/",
		}})
	}
	c.logDebug("X-Auth-Token 已同步到 cookie jar（%d 个域名）", len([]string{c.ssoBaseURL, c.baseURL}))
}
