package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── InitSession ───

// InitSession 访问登录页建立 JSESSIONID Cookie。
// 内部流程中自动调用，一般不需要外部显式调用。
func (c *Client) InitSession(ctx context.Context) error {
	url := c.ssoURL("/uiStudentLogin/login")
	if _, err := c.doBizGet(ctx, url, c.ssoHeaders()); err != nil {
		return fmt.Errorf("InitSession 失败: %w", err)
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

// OCR 重试策略常量。
// ddddocr 对同一张图是确定性的（同图重试结果完全相同），所以单图 1 次 OCR
// 即可拿到最终结果；同图重试只兜底极小概率的 CGO/IO 抖动，但收益微乎其微。
// 真正有效的是换图（新验证码字符集变化），所以把次数预算全部放在换图上：
// 1 张图 OCR 1 次 × 99 张图 = 99 次总尝试上限。
const (
	// maxOCRAttemptsPerImage 单张验证码图片 OCR 次数（ddddocr 确定性下 1 次足够）。
	maxOCRAttemptsPerImage = 1

	// maxOCRImagesTotal 最多换多少张验证码图片。
	// 1 × 99 = 99 次总尝试上限（保留原 99 次预算，分配给换图）。
	maxOCRImagesTotal = 99
)

// Login 完成 SSO 登录并返回 Token。
// 内部流程：InitSession → GetSchoolID → 多图多试 OCR（1 张图 OCR 1 次 × 最多 99 张图 = 99 次总尝试上限）→ 预校验 → 正式登录
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
	captcha, err := c.ocrRecognizeWithRetry(ctx)
	if err != nil {
		return nil, fmt.Errorf("Login OCR 自动识别验证码失败: %w", err)
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
	defer drainAndClose(httpResp.Body)

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("Login 读取响应体失败: %w", err)
	}

	// 5. 优先解析 200 JSON 响应（HAR 验证：登录响应 HTTP 200，body 含 returnData.token）
	if httpResp.StatusCode == http.StatusOK {
		var loginResp types.UnifiedResponse
		if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
			// unmarshal 失败时 logDebug 保留原始 body 上下文，便于排查非 JSON 错误响应
			c.logDebug("Login 200 响应 body 解析失败: %v body=%s", err, string(bodyBytes))
		} else {
			// 业务错误优先：code != 1 时直接返回业务 msg（如"密码错误"），
			// 避免被"未找到 token"低语义错误吞噬（修复 review-tdd finding #3）。
			if loginResp.Code != 1 {
				return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, loginResp.Code, stringPtrOr(loginResp.Msg, "登录失败"))
			}
			token, expiresAt, err := extractTokenFromReturnData(loginResp)
			if err != nil {
				// extractToken 失败时 logDebug 保留原始 body 上下文，便于排查
				// (修复 review-tdd F6: 200 路径吞掉 unmarshal/extractToken 错误)
				c.logDebug("Login 200 响应 extractToken 失败: %v body=%s", err, string(bodyBytes))
				return nil, fmt.Errorf("%w: 200 响应中未找到 token: %v", ErrLoginRejected, err)
			}
			// F1 invariant：200 路径 expiresAt 兜底（now+24h）时 warn 出来，
			// 与 302 路径 (auth.go:189-191) 语义对称。extractTokenFromReturnData
			// 当前不解析 returnData.exp/expires_in 字段，总是返回 now+24h，
			// 所以 200 路径必然走兜底——告警让用户知道 server 没给 expires 信息。
			if time.Until(expiresAt) > 23*time.Hour {
				c.logger.Warn("Login 200: returnData 未带 expires_in/exp，使用 now+24h 兜底")
			}
			// Cookie 同步：将 X-Auth-Token 写入 cookie jar，供后续业务请求使用
			// Login 路径中 token 已从 server 拿到，syncCookieToken 失败时只 Warn
			// 不阻断（业务 token 仍有效），让调用方能拿到 token 自己排查。
			c.warnSyncCookieToken(token, "200")
			return &types.LoginResponse{
				Token:     token,
				ExpiresAt: expiresAt,
				RawData:   parseRawData(bodyBytes),
			}, nil
		}
		return nil, fmt.Errorf("%w: 200 响应中未找到 token", ErrLoginRejected)
	}

	// 6. Fallback：302 Location 头提取 token
	if httpResp.StatusCode == http.StatusFound {
		location := httpResp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("%w: 302 响应中未找到 Location 头", ErrLoginRejected)
		}
		token, expiresAt := extractTokenFromLocation(location)
		if token == "" {
			return nil, fmt.Errorf("%w: Location 头中未找到 token: %s", ErrLoginRejected, location)
		}
		// 兜底 expiresAt = now+24h 时 warn 出来（说明 server 真的没给 expires）
		if time.Until(expiresAt) > 23*time.Hour {
			c.logger.Warn("Login 302 fallback: Location 未带 expires_in/exp，使用 now+24h 兜底")
		}
		// Cookie 同步
		// 302 路径同上：token 已拿到，syncCookieToken 失败只 Warn 不阻断
		c.warnSyncCookieToken(token, "302 fallback")
		return &types.LoginResponse{
			Token:     token,
			ExpiresAt: expiresAt,
			RawData:   parseRawData(bodyBytes),
		}, nil
	}

	// 非预期状态码
	var errResp types.UnifiedResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
		// unmarshal 失败时 logDebug 保留原始 body 上下文，便于排查非 JSON 错误响应
		// 修复 review-tdd finding #12：避免错误信息完全丢失根因
		c.logDebug("Login 非预期状态码 %d 响应非 JSON: %v body=%s", httpResp.StatusCode, err, string(bodyBytes))
	} else if errResp.Code != 1 {
		return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, errResp.Code, stringPtrOr(errResp.Msg, "登录失败"))
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

// ocrRecognizeWithRetry 多图多试策略识别验证码：
//
//   - 每张图片 OCR maxOCRAttemptsPerImage (1) 次（ddddocr 确定性，单次即终态）
//   - 单图失败则换新图，最多换 maxOCRImagesTotal (99) 张
//   - 任意一次 OCR 成功（非空字符串）即返回
//   - 总尝试数上限 = 1 × 99 = 99 次
//
// 关键洞察：ddddocr 引擎对同一张图是确定性的（无随机采样），同图重试只能
// 兜底极小概率的 CGO/IO 抖动；真正有效的是换图（新验证码字符集变化）。
// 把所有重试预算放在换图上，效率与原 3×33 策略等价但少 2/3 次浪费 OCR 调用。
func (c *Client) ocrRecognizeWithRetry(ctx context.Context) (string, error) {
	var lastErr error
	for imgIdx := 0; imgIdx < maxOCRImagesTotal; imgIdx++ {
		// 修复 review-tdd F11：循环顶部检查 ctx.Err()，ctx cancel 后立即返回。
		// CGO OCR 调用无法响应 ctx，但 fetchCaptchaImage 走 doBizGet 已尊重 ctx，
		// 循环顶部检查能让 ctx cancel 后不再发起新的图片请求（避免 ~99 次无意义 fetch）。
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
		// ddddocr 对同一张图是确定性的，OCR 一次即终态（maxOCRAttemptsPerImage=1），
		// 故去掉内层循环，单层结构更清晰表达"换图"语义。
		// 修复 review-tdd finding #5：消除内层死循环（结构表达意图而非假装重试）。
		text, err := c.ocr.Recognize(imgBytes)
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

// fetchCaptchaImage 拉取一张新的验证码图片。
func (c *Client) fetchCaptchaImage(ctx context.Context) ([]byte, error) {
	url := c.ssoURL("/kaptcha/kaptcha.jpg?t=" + fmt.Sprintf("%d", time.Now().UnixMilli()))
	imgBytes, err := c.doBizGet(ctx, url, c.ssoHeaders())
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
// 使用 net/url 解析，正确处理 URL encoding、fragment、复杂 query。
//
// 返回 (token, expiresAt)。expiresAt 优先级：
//  1. query 里的 expires_in=N（相对秒数）
//  2. query 里的 exp=N（绝对 Unix 时间戳，秒）
//  3. 兜底 now+24h（与原行为一致，但加 warn 日志提示）
func extractTokenFromLocation(location string) (string, time.Time) {
	u, err := url.Parse(location)
	if err != nil {
		return "", time.Now().Add(24 * time.Hour)
	}
	var token string
	if t := u.Query().Get("token"); t != "" {
		token = t
	} else if u.Fragment != "" {
		if fToken := extractTokenFromFragment(u.Fragment); fToken != "" {
			token = fToken
		}
	}
	return token, parseLocationExpires(u)
}

// parseLocationExpires 从 URL query 解析过期时间。
// 优先 expires_in（相对秒数），其次 exp（绝对 Unix 时间戳），都缺失则 now+24h。
func parseLocationExpires(u *url.URL) time.Time {
	q := u.Query()
	if v := q.Get("expires_in"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Now().Add(time.Duration(n) * time.Second)
		}
	}
	if v := q.Get("exp"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return time.Unix(n, 0)
		}
	}
	return time.Now().Add(24 * time.Hour)
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
	// Bug 3 fix：返回兜底 now+24h 而非零值 time.Time{}
	// 零值 time.Time 会被 ExpiresAt.Before(now) 误判为「已过期」
	return token, time.Now().Add(24 * time.Hour), nil
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
//
// 返回 error 让调用方感知 cookie 同步失败：
//   - 类型断言失败（非 *cookiejar.Jar，如用户自定义 http.Client 无 Jar）
//     → 返回包装错误，提示用 client.New() 默认或显式 cookiejar.New(nil)
//   - base URL 解析失败 → 返回包装错误（review-tdd F5，invariant 对称性补全）
//
// F5 修复动机：F8 round1 修了 Jar 类型断言失败 propagate error，但 baseURL
// 解析失败仍 c.logger.Warn + continue + return nil。两条失败路径契约不
// 对称：调用方在 build client 阶段只能感知类型断言失败，对畸形 baseURL
// 毫无感知（Warn 默认 LevelWarn 静默，业务 URL 仍可控）。改为 propagate
// error 后，New() 路径可在 build 阶段拒绝畸形配置，warnSyncCookieToken
// helper 继续 WARN 不阻断（业务 token 仍有效，调用方能拿到 token 自己排查）。
func (c *Client) syncCookieToken(token string) error {
	jar, ok := c.http.Jar.(*cookiejar.Jar)
	// 修复 review-tdd F8：类型断言失败时返回 error 而不是仅 Warn。
	// WithHTTPClient 自定义 Jar（非 *cookiejar.Jar）时 X-Auth-Token 同步失败，
	// 业务接口返回空 dataList 但根因在 build client 阶段的 stderr Warn，
	// 跨多步调用难关联。让 New() propagate error 让调用方立即拿到根因。
	if !ok {
		return fmt.Errorf("syncCookieToken: HTTP client 的 Jar 不是 *cookiejar.Jar（实际类型 %T），X-Auth-Token 无法同步到 cookie。"+
			"修复：用 client.New() 默认 HTTP 客户端，或显式 &http.Client{Jar: cookiejar.New(nil)} 创建",
			c.http.Jar)
	}
	for _, raw := range []string{c.ssoBaseURL, c.baseURL} {
		u, err := url.Parse(raw)
		if err != nil {
			// 修复 review-tdd F5：URL 解析失败 propagate error（与 Jar 类型断言
			// 失败契约对称）。成功循环计数改用 len(URLs) - 失败次数，调用方可在
			// build 阶段感知畸形 baseURL。
			return fmt.Errorf("syncCookieToken: 解析 base URL %q 失败: %w", raw, err)
		}
		jar.SetCookies(u, []*http.Cookie{{
			Name:  "X-Auth-Token",
			Value: token,
			Path:  "/",
		}})
	}
	c.logDebug("X-Auth-Token 已同步到 cookie jar（%d 个域名）", len([]string{c.ssoBaseURL, c.baseURL}))
	return nil
}

// warnSyncCookieToken 是 Login 200/302 路径共用的 syncCookieToken 包装器。
//
// 提取动机：200 路径和 302 路径过去都是 copy-paste 的 "if err := c.syncCookieToken;
// err != nil { c.logger.Warn(...) }"，语义相同（token 已拿到，cookie 同步失败只
// Warn 不阻断），改 helper 后只需保证语义一致。
//
// 失败语义：syncCookieToken 返回 error 时输出 WARN（默认 LevelWarn 下用户可见），
// 不阻断 Login 主流程（业务 token 仍有效，调用方能拿到 token 自己排查）。
//
// label 用于在日志中标识路径来源（如 "200"、"302 fallback"），便于排查时定位
// 触发点。注意不要把 token 写入日志（避免泄露敏感凭据）。
func (c *Client) warnSyncCookieToken(token, label string) {
	if err := c.syncCookieToken(token); err != nil {
		c.logger.Warn("Login "+label+" 后同步 token 到 cookie 失败", "err", err.Error())
	}
}
