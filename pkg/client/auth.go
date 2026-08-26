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

	"github.com/Wenaixi/nazhi-cli/pkg/logx"
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

	schools, err := types.DecodeDataList[map[string]any](resp)
	if err != nil {
		return nil, fmt.Errorf("GetSchoolID dataList 解析失败: %w", err)
	}

	if len(schools) == 0 {
		return nil, fmt.Errorf("GetSchoolID: 未找到学校信息")
	}

	school := schools[0]

	// 校验 school_id 为有效数字，防止非数字值被静默传给登录请求
	schoolIDRaw, ok := school["school_id"]
	if !ok || schoolIDRaw == nil {
		return nil, fmt.Errorf("%w: GetSchoolID school_id 字段缺失或为 nil", ErrInvalidPayload)
	}
	schoolIDStr := fmt.Sprintf("%v", schoolIDRaw)
	if _, err := strconv.ParseInt(schoolIDStr, 10, 64); err != nil {
		return nil, fmt.Errorf("%w: GetSchoolID school_id=%q 不是有效数字: %w", ErrInvalidPayload, schoolIDStr, err)
	}
	schoolName := ""
	// P2-3：学校名键双兼容——服务端 school_id 用小写键、NAME 用大写键，风格不一致；
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

// ─── Login ───

const (
	// maxOCRImagesTotal 是总 OCR 尝试次数上限。
	// 视觉识别器对同图结果通常稳定，失败时每次尝试都更换验证码图片。
	// 多数场景 1-3 张即可成功。
	maxOCRImagesTotal        = 9
	expiresFallbackThreshold = 1 * time.Hour
)

// ocrTimeout 是 OCR 自动超时时长。
// 定义为 var 而非 const，允许测试中覆写以加速测试。
var ocrTimeout = 120 * time.Second

// Login 完成 SSO 登录并返回 Token。
//
// GetSchoolID 与 OCR 验证码识别无数据依赖，通过 errgroup 并发执行。
// InitSession 必须在两者之前完成（需要先建立 JSESSIONID Cookie）。
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

	// 步骤 2&3: GetSchoolID + OCR 识别并发进行（两者无数据依赖）
	schoolID := req.SchoolID
	var captcha string

	g, gctx := errgroup.WithContext(ctx)

	if schoolID == "" {
		g.Go(func() error {
			info, err := c.GetSchoolID(gctx, req.Username)
			if err != nil {
				return fmt.Errorf("Login GetSchoolID 失败: %w", err)
			}
			schoolID = info.SchoolID
			return nil
		})
	}

	g.Go(func() error {
		var err error
		captcha, err = c.ocrRecognizeWithRetry(gctx)
		if err != nil {
			return fmt.Errorf("Login OCR 自动识别验证码失败: %w", err)
		}
		c.logDebugCtx(ctx, "OCR 识别完成（%d 字符）", len(captcha))
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
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
	bodySnippet := logx.RedactBodyThenTruncate(bodyBytes, 100)

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
		return c.buildLoginResponse(token, expiresAt, bodyBytes, "200"), nil
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
		return c.buildLoginResponse(token, expiresAt, bodyBytes, "302 fallback"), nil
	}

	errResp, err := types.DecodeResponse(bodyBytes)
	if err != nil {
		c.logDebugCtx(ctx, "Login 非预期状态码 %d 响应非 JSON: %v body=%s", httpResp.StatusCode, err, bodySnippet)
	} else if err := types.CheckCode(errResp); err != nil {
		return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, errResp.Code, types.DerefOr(errResp.Msg, "登录失败"))
	}
	// 错误消息附带 RedactBodyThenTruncate 截断脱敏摘要：非预期状态码的典型场景是 nginx 503、
	// CDN challenge 等 HTML 响应；不带 body 片段时用户难以定位根因。
	// 摘要再过 RedactBody 与 request.go 同类分支脱敏口径拉平（90ccd64 先例）。
	return nil, fmt.Errorf("%w: 非预期状态码 %d body=%s",
		ErrLoginRejected, httpResp.StatusCode, logx.RedactBodyThenTruncate(bodyBytes, 100))
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
	if remaining > tokenparse.DefaultTokenTTL-2*expiresFallbackThreshold &&
		remaining < tokenparse.DefaultTokenTTL+expiresFallbackThreshold {
		c.logger.Warn("Login token 剩余寿命恰好 ≈24h，服务器可能未带 expires_in/exp",
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

	resp, err := decodeOrInvalidResponse("验证码预校验", bodyBytes)
	if err != nil {
		return err
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
//
// 单通道 OCR：直接用 c.ocr 识别，不分 primary/fallback 级联。
// 最多尝试 maxOCRImagesTotal 张图，每张图 OCR 一次后立即 validateCaptcha 预校验。
func (c *Client) ocrRecognizeWithRetry(ctx context.Context) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ocrTimeout)
		defer cancel()
	}

	return c.ocrRetryLoop(ctx, c.safeOCRRecognize)
}

// ocrRetryLoop 执行一轮 OCR 重试循环（最多 maxOCRImagesTotal 张图）。
// recognizeFn 是实际的识别函数。
func (c *Client) ocrRetryLoop(ctx context.Context, recognizeFn func([]byte) (string, error)) (string, error) {
	var lastErr error
	for imgIdx := 0; imgIdx < maxOCRImagesTotal; imgIdx++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			c.logDebugCtx(ctx, "OCR 循环顶部检测到 ctx cancel（img=%d）: %v", imgIdx+1, ctxErr)
			return "", fmt.Errorf("OCR 识别被 ctx cancel（已重试 %d 次）: %w", imgIdx, ctxErr)
		}
		imgBytes, err := c.fetchCaptchaImage(ctx)
		if err != nil {
			lastErr = err
			c.logDebugCtx(ctx, "OCR 获取第 %d 张验证码失败: %v", imgIdx+1, err)
			continue
		}
		text, err := recognizeFn(imgBytes)
		switch {
		case err != nil:
			lastErr = err
			c.logDebugCtx(ctx, "OCR 第 %d 张图失败: %v", imgIdx+1, err)
		case text == "":
			lastErr = fmt.Errorf("空白结果")
			c.logDebugCtx(ctx, "OCR 第 %d 张图结果为空白", imgIdx+1)
		default:
			c.logDebugCtx(ctx, "OCR 识别成功: img=%d result_len=%d", imgIdx+1, len(text))
			// 验证码预校验：服务端确认该验证码有效后再返回。
			// 校验失败（code≠1）不是 OCR 读错了，而是服务端不认这张图的验证码，
			// 需要换图重试。
			if err := c.validateCaptcha(ctx, text); err != nil {
				lastErr = err
				c.logDebugCtx(ctx, "验证码校验失败(img=%d): %v", imgIdx+1, err)
				continue
			}
			return text, nil
		}
	}
	return "", fmt.Errorf("OCR 识别 %d 张图均失败（共 %d 次尝试），最后错误: %w",
		maxOCRImagesTotal, maxOCRImagesTotal, lastErr)
}

var captchaSeq atomic.Int64

// fetchCaptchaImage 拉取一张新的验证码图片。
//
// seq 原子计数防缓存碰撞，查询串经 url.Values 编码。
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
