package client

import (
	"context"
	"encoding/json"
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
	schoolID = fmt.Sprintf("%v", school["school_id"])
	if v, ok := school["NAME"]; ok {
		schoolName = fmt.Sprintf("%v", v)
	}

	return schoolID, schoolName, nil
}

// ─── Login ───

const (
	maxOCRAttemptsPerImage   = 1
	maxOCRImagesTotal        = 99
	ocrTimeout               = 30 * time.Second
	defaultTokenTTL          = 24 * time.Hour
	expiresFallbackThreshold = 1 * time.Hour
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

	httpResp, err := c.rawDoWithResp(ctx, http.MethodPost,
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
			return nil, fmt.Errorf("%w: 响应 body JSON 解析失败: %v", ErrLoginRejected, err)
		}
		if loginResp.Code != 1 {
			return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, loginResp.Code, types.DerefOr(loginResp.Msg, "登录失败"))
		}
		if loginResp.ReturnData == nil {
			c.logDebug("Login 200 响应 returnData 为空 body=%s", string(bodyBytes))
			return nil, fmt.Errorf("%w: 200 响应中未找到 token", ErrLoginRejected)
		}
		// 检查 returnData 是否为 JSON null 字面量（{"returnData": null}），
		// 避免误报"token 字段类型异常"。
		if len(*loginResp.ReturnData) == 4 && string(*loginResp.ReturnData) == "null" {
			c.logDebug("Login 200 响应 returnData 为 null body=%s", string(bodyBytes))
			return nil, fmt.Errorf("%w: returnData 为 null", ErrLoginRejected)
		}
		token, expiresAt, err := tokenparse.ExtractFromReturnData(*loginResp.ReturnData)
		if err != nil {
			c.logDebug("Login 200 响应 extractToken 失败: %v body=%s", err, string(bodyBytes))
			return nil, fmt.Errorf("%w: 200 响应中未找到 token: %v", ErrLoginRejected, err)
		}
		if time.Until(expiresAt) > defaultTokenTTL-expiresFallbackThreshold {
			c.logger.Warn("Login 200: returnData 未带 expires_in/exp，使用 now+24h 兜底")
		}
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
			return nil, fmt.Errorf("%w: Location 头解析失败: %v", ErrLoginRejected, locErr)
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
		return nil, fmt.Errorf("%w: code=%d msg=%s", ErrLoginRejected, errResp.Code, types.DerefOr(errResp.Msg, "登录失败"))
	}
	return nil, fmt.Errorf("%w: 非预期状态码 %d", ErrLoginRejected, httpResp.StatusCode)
}

// ─── 验证码内部辅助 ───

func (c *Client) validateCaptcha(ctx context.Context, captcha string) error {
	bodyBytes, err := c.httpDo(ctx, http.MethodPost,
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
// 删除冗余的 t= 时间戳参数（seq 原子计数器已足够防缓存碰撞），
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
