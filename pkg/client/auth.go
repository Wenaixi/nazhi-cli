package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/tokenparse"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"golang.org/x/sync/errgroup"
)

// ─── InitSession ───

// InitSession 访问登录页建立 JSESSIONID Cookie。
// 内部流程中自动调用，一般不需要外部显式调用。
func (c *Client) InitSession(ctx context.Context) error {
	u := c.ssoURL("/uiStudentLogin/login", nil)
	if _, err := c.doBizGet(ctx, u, c.ssoHeaders()); err != nil {
		return fmt.Errorf("InitSession 失败: %w", err)
	}
	return nil
}

// ─── GetSchoolID ───

// GetSchoolID 根据学号查询学校 ID 和学校名称。
func (c *Client) GetSchoolID(ctx context.Context, username string) (schoolID string, schoolName string, err error) {
	u := c.ssoURL("/teacher/auth/studentLogin/getSchoolIdByStudentNumber", url.Values{"userName": {username}})

	headers := c.ssoHeaders()
	headers["Referer"] = c.ssoURL("/uiStudentLogin/login", url.Values{"userName": {username}})

	bodyBytes, err := c.httpDo(ctx, http.MethodPost, u, map[string]string{"key": ""}, headers, "application/json")
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

	// 校验 school_id 为有效数字，防止非数字值被静默传给登录请求
	schoolIDRaw, ok := school["school_id"]
	if !ok || schoolIDRaw == nil {
		return "", "", fmt.Errorf("%w: GetSchoolID school_id 字段缺失或为 nil", ErrInvalidPayload)
	}
	schoolIDStr := fmt.Sprintf("%v", schoolIDRaw)
	if _, err := strconv.ParseInt(schoolIDStr, 10, 64); err != nil {
		return "", "", fmt.Errorf("%w: GetSchoolID school_id=%q 不是有效数字: %w", ErrInvalidPayload, schoolIDStr, err)
	}
	schoolID = schoolIDStr
	if v, ok := school["NAME"]; ok {
		schoolName = fmt.Sprintf("%v", v)
	}

	return schoolID, schoolName, nil
}

// ─── Login ───

const (
	// maxOCRImagesTotal 是总 OCR 尝试次数上限。
	// ddddocr 对同图识别是确定性的，重试同图无意义，每次尝试都换新图。
	maxOCRImagesTotal        = 99
	expiresFallbackThreshold = 1 * time.Hour
)

// ocrTimeout 是 OCR 自动超时时长。
// 定义为 var 而非 const，允许测试中覆写以加速测试。
var ocrTimeout = 30 * time.Second

// Login 完成 SSO 登录并返回 Token。
//
// F5 优化：GetSchoolID 和 OCR 验证码识别无数据依赖，通过 errgroup 并发执行。
// InitSession 必须在 OCR 之前完成（需要先建立 JSESSIONID Cookie）。
func (c *Client) Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error) {
	if c.ocr == nil {
		return nil, ErrOCRNotConfigured
	}
	if c.http == nil {
		return nil, fmt.Errorf("Login 失败: HTTP 客户端为 nil，无法发送请求")
	}

	// 步骤 1: InitSession（串行前置，必须最先建立 JSESSIONID）
	if err := c.InitSession(ctx); err != nil {
		return nil, fmt.Errorf("Login InitSession 失败: %w", err)
	}

	// 步骤 2&3: GetSchoolID + OCR 识别并发进行（F5 无数据依赖）
	schoolID := req.SchoolID
	var captcha string

	g, gctx := errgroup.WithContext(ctx)

	if schoolID == "" {
		g.Go(func() error {
			sid, _, err := c.GetSchoolID(gctx, req.Username)
			if err != nil {
				return fmt.Errorf("Login GetSchoolID 失败: %w", err)
			}
			schoolID = sid
			return nil
		})
	}

	g.Go(func() error {
		var err error
		captcha, err = c.ocrRecognizeWithRetry(gctx)
		if err != nil {
			return fmt.Errorf("Login OCR 自动识别验证码失败: %w", err)
		}
		c.logDebug("OCR 识别完成（%d 字符）", len(captcha))
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 步骤 4: 验证码预校验（依赖 OCR 结果，串行执行）
	if err := c.validateCaptcha(ctx, captcha); err != nil {
		return nil, fmt.Errorf("Login 验证码预校验未通过: %w", err)
	}

	loginBody := map[string]string{
		"schoolId": schoolID,
		"username": req.Username,
		"password": req.Password,
	}

	httpResp, err := c.rawDoWithResp(ctx, http.MethodPost,
		c.ssoURL("/teacher/auth/studentLogin/validate", nil),
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
	bodySnippet := logSafeBody(bodyBytes)

	if httpResp.StatusCode == http.StatusOK {
		loginResp, err := types.DecodeResponse(bodyBytes)
		if err != nil {
			c.logDebug("Login 200 响应 body 解析失败: %v body=%s", err, bodySnippet)
			return nil, fmt.Errorf("%w: 响应 body JSON 解析失败: %w", ErrLoginRejected, err)
		}
		if err := types.CheckCode(loginResp); err != nil {
			return nil, fmt.Errorf("登录失败: %w", errors.Join(ErrLoginRejected, err))
		}
		if loginResp.ReturnData == nil || bytes.Equal(bytes.TrimSpace(*loginResp.ReturnData), []byte("null")) {
			c.logDebug("Login 200 响应 returnData 为 null body=%s", bodySnippet)
			return nil, fmt.Errorf("%w: returnData 为 null", ErrLoginRejected)
		}
		token, expiresAt, err := tokenparse.ExtractFromReturnData(*loginResp.ReturnData)
		if err != nil {
			c.logDebug("Login 200 响应 extractToken 失败: %v body=%s", err, bodySnippet)
			return nil, fmt.Errorf("%w: 200 响应中未找到 token: %w", ErrLoginRejected, err)
		}
		c.warnIfExpiresAtFallback(expiresAt, "200")
		return c.buildLoginResponse(token, expiresAt, bodyBytes, "200"), nil
	}

	if httpResp.StatusCode == http.StatusFound {
		location := httpResp.Header.Get("Location")
		if location == "" {
			return nil, fmt.Errorf("%w: 302 响应中未找到 Location 头", ErrLoginRejected)
		}
		token, expiresAt, locErr := tokenparse.ExtractFromLocation(location)
		if locErr != nil {
			c.logDebug("Login 302: Location 头解析失败: %v location=%s", locErr, location)
			return nil, fmt.Errorf("%w: Location 头解析失败: %w", ErrLoginRejected, locErr)
		}
		if token == "" {
			return nil, fmt.Errorf("%w: Location 头中未找到 token: %s", ErrLoginRejected, location)
		}
		c.warnIfExpiresAtFallback(expiresAt, "302 fallback")
		return c.buildLoginResponse(token, expiresAt, bodyBytes, "302 fallback"), nil
	}

	errResp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		c.logDebug("Login 非预期状态码 %d 响应非 JSON: %v body=%s", httpResp.StatusCode, err, bodySnippet)
	} else if err := types.CheckCode(errResp); err != nil {
		return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, errResp.Code, types.DerefOr(errResp.Msg, "登录失败"))
	}
	// F4 修复：非预期状态码错误消息附 logSafeBody(bodyBytes) 截断摘要（100 字节）。
	// 修复前错误消息仅含 "非预期状态码 %d"，body 信息丢给 c.logDebug（默认 LevelWarn
	// 下被静默过滤），用户必须开 verbose 才能定位。修复后错误消息直接带 body 片段，
	// 用户无论 verbose 与否都能在打印的 error 上看到原始响应摘要（典型 nginx 503
	// HTML、CDN challenge、HTML 错误页等）。
	return nil, fmt.Errorf("%w: 非预期状态码 %d body=%s",
		ErrLoginRejected, httpResp.StatusCode, logSafeBody(bodyBytes))
}

// warnIfExpiresAtFallback 在 expiresAt 异常时输出 WARN 日志。两条 Login 路径
// （200/302）共用，避免重复。
//
// 检测两类异常:
//
//  1. fallback 触发：剩余寿命 > defaultTokenTTL-threshold（典型 24h 兜底），
//     意味着 server 没带 expires_in/exp。
//  2. 已过期/即将过期：剩余寿命 < expiresFallbackThreshold，server 给的 exp
//     已是过去时间（或剩余过短），首次业务调用会立即 401。
//
// F4 修复前：只检测 (1)，过去时间 time.Until 为负数不大于 23h → 静默吞下。
// F4 修复后：合并 (1) + (2)，两条都覆盖。
func (c *Client) warnIfExpiresAtFallback(expiresAt time.Time, label string) {
	remaining := time.Until(expiresAt)
	if remaining > tokenparse.DefaultTokenTTL-expiresFallbackThreshold {
		c.logger.Warn("Login token 剩余寿命过长，server 可能未带 expires_in/exp，使用 now+24h 兜底",
			"label", label,
			"remaining", remaining.Round(time.Second),
			"expiresAt", expiresAt.Format(time.RFC3339))
		return
	}
	if remaining < expiresFallbackThreshold {
		c.logger.Warn("Login token 已过期或剩余 < 1h，首次业务调用将立即 401",
			"label", label,
			"remaining", remaining.Round(time.Second),
			"expiresAt", expiresAt.Format(time.RFC3339))
	}
}

// ─── 验证码内部辅助 ───

func (c *Client) validateCaptcha(ctx context.Context, captcha string) error {
	bodyBytes, err := c.httpDo(ctx, http.MethodPost,
		c.ssoURL("/uiStudentLogin/validateCaptcha", nil),
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
		// 验证码校验失败属于 Login 流程的错误（不是普通业务 API 拒绝），
		// 包装 ErrLoginRejected 而非 ErrBusinessRejected，让 SDK 用户
		// 用 errors.Is(err, ErrLoginRejected) 能命中。
		return fmt.Errorf("验证码校验失败: %w", errors.Join(ErrLoginRejected, err))
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
			c.logDebug("OCR 识别成功: img=%d result_len=%d", imgIdx+1, len(text))
			return text, nil
		}
	}
	return "", fmt.Errorf("OCR 识别 %d 张图均失败（共 %d 次尝试），最后错误: %w",
		maxOCRImagesTotal, maxOCRImagesTotal, lastErr)
}

var captchaSeq atomic.Int64

// fetchCaptchaImage 拉取一张新的验证码图片。
//
// 删除冗余的 t= 时间戳参数（seq 原子计数器已足够防缓存碰撞），
// 改用 url.Values 编码替代 fmt.Sprintf+strconv.FormatInt 混合拼接风格。
func (c *Client) fetchCaptchaImage(ctx context.Context) ([]byte, error) {
	seq := captchaSeq.Add(1)
	u := c.ssoURL("/kaptcha/kaptcha.jpg", url.Values{"seq": {strconv.FormatInt(seq, 10)}})
	imgBytes, err := c.doBizGet(ctx, u, c.ssoHeaders())
	if err != nil {
		return nil, fmt.Errorf("获取验证码图片失败: %w", err)
	}

	if len(imgBytes) == 0 {
		return nil, fmt.Errorf("获取验证码图片响应为空 status=200")
	}
	return imgBytes, nil
}

// ssoURL 拼接 SSO 域名的完整 URL。
// 与 bizURL helper 对称，统一管理 SSO URL 拼接。
func (c *Client) ssoURL(path string, q url.Values) string {
	if len(q) > 0 {
		return c.ssoBaseURL + path + "?" + q.Encode()
	}
	return c.ssoBaseURL + path
}
