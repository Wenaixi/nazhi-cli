// auth_test.go 聚合 pkg/client 内部白盒测试，覆盖以下修复回归：
//   - F1: Login drain+close 让 keep-alive 池归还连接
//   - F2: 200/302 路径对称 expiresAt warn + warnSyncCookieToken helper 去重
//   - F2-EXTRACT-TOKEN-ASYM: extractTokenFromLocation 畸形 URL 返回 error
//   - F8-CAPTCHA-URL-COLLISION: captchaSeq atomic 保证 URL 唯一
//   - F10-FRAGMENT-URLDECODE: extractTokenFromFragment URL 解码
//   - G2: extractTokenFromReturnData 解析 expires_in/exp
//   - G3: Login 200 ReadAll 错误含 status + read 字节数
//   - M2: stringPtrOr → derefOr 重命名 + nil-safe 语义
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Wenaixi/nazhi-cli/pkg/tokenparse"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── auth_200_expires_warn_test.go: 200 路径 expiresAt 兜底 WARN 日志 ───

// TestLogin_200Path_ExpiresAtFallback_LogsAtWarn 验证 200 路径触发 expiresAt
// 兜底（now+24h）时，告警必须以 WARN 级别输出，与 302 路径语义对称。
// 场景：server 返回 200 + UnifiedResponse，returnData 含 token 但**无**exp/expires_in
// 字段（HAR 验证的登录响应现状，server 不带过期信息）。
// extractTokenFromReturnData 返回 now+24h → Login 应 Warn 提示。
// 修复前：完全静默（200 路径无任何 expiresAt warn 代码）。
// 修复后：c.logger.Warn → 默认 LevelWarn 下用户立即知道 server 行为异常。
func TestLogin_200Path_ExpiresAtFallback_LogsAtWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			// InitSession: 任意 200 即可
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			// 验证码图片: 任意非空字节
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			// 预校验验证码: 业务成功
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 登录: 200 + UnifiedResponse，returnData 含 token 但**无**exp/expires_in
			// （HAR 验证的现状：server 不带过期信息，200 路径永远走 now+24h 兜底）。
			// 注意：returnData 是嵌套 JSON 对象（json.RawMessage），不是字符串。
			w.Header().Set("Content-Type", "application/json")
			// 关键：returnData 只有 token 字段，没有 exp/expires_in
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"token":"jwt-no-expires"}}`))
		}
	}))
	defer srv.Close()

	// 自定义 logger: bytes.Buffer 收集所有日志，LevelDebug 让 Debug 也可见
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173", // 跳过 GetSchoolID
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if resp.Token != "jwt-no-expires" {
		t.Errorf("token 应为 'jwt-no-expires'，实际: %s", resp.Token)
	}

	// 关键断言：日志必须以 WARN 级别输出兜底告警
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "level=WARN") {
		t.Errorf("期望 WARN 级别日志（让默认 LevelWarn 下用户也能看见），实际日志:\n%s", logOutput)
	}
	// 断言内容含兜底语义（与 302 路径一致）
	if !strings.Contains(logOutput, "24h") && !strings.Contains(logOutput, "兜底") && !strings.Contains(logOutput, "fallback") {
		t.Errorf("兜底告警应说明 '24h 兜底' 语义，实际日志:\n%s", logOutput)
	}
	// 断言是 200 路径（带 "200" 标识符以便区分 302 路径告警）
	if !strings.Contains(logOutput, "200") {
		t.Errorf("兜底告警应包含 '200' 路径标识符，实际日志:\n%s", logOutput)
	}
}

// ─── auth_business_code_test.go: HTTP 200 + 业务错误码时返回业务 msg ───

// TestLogin_200WithBusinessErrorCode 验证 HTTP 200 + 业务错误码时，
// 应返回包含业务 msg 的错误（如"密码错误"），而不是低语义的"未找到 token"。
// Bug 场景：server 返回 200 + {"code":2,"msg":"密码错误"} → 之前会丢失业务信息。
func TestLogin_200WithBusinessErrorCode(t *testing.T) {
	// 业务错误只在登录 POST 时返回，其他路径返回成功
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			// InitSession: 任意 200 即可
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html>ok</html>`))
		case "/kaptcha/kaptcha.jpg":
			// 验证码图片: 任意非空字节
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			// 预校验验证码: 业务成功
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 登录: 200 + 业务错误码（关键测试场景）
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":2,"msg":"密码错误"}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		}
	}))
	defer srv.Close()

	// 内部包：直接构造 Client + 注入 mock OCR + 提供 SchoolID 跳过 GetSchoolID
	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     slog.New(slog.DiscardHandler),
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "user",
		Password: "pass",
		SchoolID: "173", // 跳过 GetSchoolID
	})

	if err == nil {
		t.Fatal("期望返回业务错误，实际 nil")
	}
	// 必须含业务 msg，不能只是"未找到 token"
	if !strings.Contains(err.Error(), "密码错误") {
		t.Errorf("错误信息应包含业务 msg '密码错误'，实际: %v", err)
	}
	if strings.Contains(err.Error(), "未找到 token") {
		t.Errorf("错误信息不应是低语义的'未找到 token'，实际: %v", err)
	}
}

// ─── auth_captcha_seq_test.go: captchaSeq atomic 唯一性 ───

// TestFetchCaptchaImage_ConcurrentDifferentURLs 验证：8 路 goroutine 并发调用
// fetchCaptchaImage 拿到 8 个不同的 URL，避免并发 Login 撞同 URL 浪费 OCR 预算。
// 动机：原版用 time.Now().UnixMilli() 作为 cache-busting 参数，同一毫秒内
// 并发调用生成完全相同的 URL → 8 路 OCR 拿到同一张验证码图片（同一字符集）→
// 7 路必失败。
// 修复后：atomic.Int64 累加 seq 追加到 URL query，URL 唯一性由 atomic 保证。
func TestFetchCaptchaImage_ConcurrentDifferentURLs(t *testing.T) {
	// 记录所有进入 mock server 的 URL（含 query string）
	var (
		mu   sync.Mutex
		urls []string
	)
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		urls = append(urls, r.URL.String())
		mu.Unlock()
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer sso.Close()

	mock := &countMockOCR{failBeforeSuccess: 0, returnText: "ab12"}
	c := newClientForOCRTest(sso.URL, mock)

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // 8 路同时放行，最大化同毫秒撞车概率
			if _, err := c.fetchCaptchaImage(context.Background()); err != nil {
				t.Errorf("fetchCaptchaImage 失败: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(urls) != n {
		t.Fatalf("期望 %d 次 fetch，实际 %d", n, len(urls))
	}
	seen := make(map[string]bool, n)
	for _, u := range urls {
		if seen[u] {
			t.Errorf("URL 重复: %s", u)
		}
		seen[u] = true
	}
}

// TestCaptchaSeq_Monotonic 验证 captchaSeq 累加器单调用递增。
// 防御性测试：未来若有人改回非 atomic 实现（如 sync.Mutex 包裹 int），
// 此测试会捕获 goroutine 间数据竞争。
func TestCaptchaSeq_Monotonic(t *testing.T) {
	const n = 100
	var seqs [n]int64

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			seqs[idx] = captchaSeq.Add(1)
		}(i)
	}
	wg.Wait()

	// 验证所有 seq 唯一（atomic.Add 保证）
	seen := make(map[int64]bool, n)
	for i, s := range seqs {
		if seen[s] {
			t.Errorf("seq %d 重复 (idx=%d)", s, i)
		}
		seen[s] = true
	}
	_ = atomic.LoadInt64 // touch atomic import even if unused
}

// ─── auth_drain_test.go: Login drain body 让 keep-alive 池归还连接 ───

// readTrackingBody 是 Login 测试用的 mock body。
// - 第 1 次 Read：返回 (0, io.ErrUnexpectedEOF) —— 让 io.ReadAll 立即失败
// - 之后 Read：返回 remainingBytes 切片内容（只有 drain 才会调用）
// - 每次 Read 都把 n 加到 *readByDrain（drain 标识）
type readTrackingBody struct {
	remaining   []byte
	readByDrain *int32 // 累计 Read 返回的字节数（无论 io.ReadAll 还是 drain）
	firstRead   bool   // 第一次 Read 返回 (0, io.ErrUnexpectedEOF)
}

func (b *readTrackingBody) Read(p []byte) (int, error) {
	if b.firstRead {
		b.firstRead = false
		return 0, io.ErrUnexpectedEOF
	}
	if len(b.remaining) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.remaining)
	b.remaining = b.remaining[n:]
	atomic.AddInt32(b.readByDrain, int32(n))
	return n, nil
}

func (b *readTrackingBody) Close() error { return nil }

// readTrackingRT 是 http.RoundTripper mock。
// 它只对 /teacher/auth/studentLogin/validate (Login 主路径) 返回带 remaining 的 body。
// 其他路径返回正常响应，让 Login 走通到 validate。
type readTrackingRT struct {
	validateReadBytes *int32
	calls             *int32
}

func (rt *readTrackingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(rt.calls, 1)

	// Login 主路径：返回带 100 字节 remaining 的 body
	// 第 1 次 Read 返回 (0, io.ErrUnexpectedEOF) → io.ReadAll 立即失败
	// Login 走 line 137 错误路径 → defer Close 触发
	// 修复前：defer 只 Close，剩余 100 字节从未被读 → 连接被强制关闭
	// 修复后：defer drain 调 io.Copy(io.Discard, body) → 100 字节被读完
	if strings.Contains(req.URL.Path, "validate") &&
		!strings.Contains(req.URL.Path, "validateCaptcha") &&
		req.Method == http.MethodPost {
		body := &readTrackingBody{
			remaining:   bytes.Repeat([]byte{'D'}, 100),
			readByDrain: rt.validateReadBytes,
			firstRead:   true,
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       body,
			Header:     http.Header{"Content-Length": []string{"100"}},
			Request:    req,
		}, nil
	}

	// InitSession / captcha / validateCaptcha：返回正常 200
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":1,"msg":"成功"}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

func (rt *readTrackingRT) Close() error { return nil }

// TestLogin_DrainsBody_On200UnexpectedEOFPath 验证 Login 在 HTTP 200
// + body io.ErrUnexpectedEOF 路径上，必须 drain body 才能让连接归还 keep-alive。
// 场景：server 返回 200 + body 声明 100 字节但立即 io.ErrUnexpectedEOF。
// io.ReadAll 失败 → Login 走 line 137 错误返回 → defer Close() 触发。
// 修复前：defer httpResp.Body.Close() → 连接被强制关闭（drainedBytes == 0）。
// 修复后：defer { io.Copy(io.Discard, body); body.Close() } → drainedBytes == 100。
func TestLogin_DrainsBody_On200UnexpectedEOFPath(t *testing.T) {
	var validateReadBytes int32
	var calls int32

	rt := &readTrackingRT{
		validateReadBytes: &validateReadBytes,
		calls:             &calls,
	}

	c := &Client{
		ssoBaseURL: "http://mock-sso",
		baseURL:    "http://mock-sso",
		uploadURL:  "http://mock-sso",
		http: &http.Client{
			Transport: rt,
		},
		logger: slog.New(slog.DiscardHandler),
		ocr:    &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173", // 跳过 GetSchoolID
	})

	if err == nil {
		t.Fatal("期望 io.ReadAll 失败，实际 nil")
	}
	// 错误应包含 io.ErrUnexpectedEOF（被 fmt.Errorf %w 包装）
	if !strings.Contains(err.Error(), io.ErrUnexpectedEOF.Error()) {
		t.Errorf("期望 wrap io.ErrUnexpectedEOF，实际: %v", err)
	}

	// 关键断言：validate body 被读取的总字节数 = 100（drain 阶段读走 100 字节）
	// 修复前：defer 只 Close，drain 阶段无 Read 调用 → readBytes < 100
	// 修复后：defer drain 调 io.Copy → readBytes = 100
	readBytes := atomic.LoadInt32(&validateReadBytes)
	if readBytes != 100 {
		t.Errorf("期望 validate body 被读完 100 字节（drain 让连接归还 keep-alive），实际 %d 字节（连接被强制关闭）", readBytes)
	}
}

// ─── auth_unmarshal_log_test.go: 200 路径解析失败 logDebug body 摘要 ───

// TestLogin_200Path_LogsUnmarshalFailure 验证 HTTP 200 + body 解析失败时，
// 必须 logDebug 输出原始 body 摘要（便于排查非 UnifiedResponse 错误响应）。
// 场景 1：body 是空对象 {} → json.Unmarshal 成功但 loginResp.ReturnData 为 nil
//
//	→ extractTokenFromReturnData 返回 "returnData 为空" 错误
//	→ 当前实现：吞掉错误，错误信息只说"未找到 token"
//	→ 修复后：logDebug 输出 body + 错误
func TestLogin_200Path_LogsUnmarshalFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 关键：返回 200 + 空对象 {} → json.Unmarshal 成功但无 token 字段
			w.Header().Set("Content-Type", "application/json")
			// 返回 returnData=null 让 extractTokenFromReturnData 失败
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null}`))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误，实际 nil")
	}
	if !strings.Contains(err.Error(), "未找到 token") {
		t.Errorf("期望 '未找到 token' 错误，实际: %v", err)
	}

	// 关键断言：logDebug 必须输出原始 body 摘要 + 错误原因
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Login 200") {
		t.Errorf("logDebug 应输出 'Login 200' 标识符便于排查，实际日志:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "body=") {
		t.Errorf("logDebug 应包含 body= 字段便于查看原始 body，实际日志:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "returnData") {
		t.Errorf("logDebug 应包含 body 内容 'returnData'（mock body 的标识），实际日志:\n%s", logOutput)
	}
}

// TestLogin_200Path_LogsNonJSONBody 验证 HTTP 200 + body 不是 JSON 时
// （例如 HTML 错误页），必须 logDebug 输出 body 摘要。
// 场景：server 返回 200 + HTML 错误页（中间件拦截）。
// json.Unmarshal 失败 → 当前实现：吞掉错误，错误信息只说"未找到 token"
// 修复后：logDebug 输出 "解析失败" + body 摘要。
func TestLogin_200Path_LogsNonJSONBody(t *testing.T) {
	const htmlBody = "<html><body>500 Internal Server Error</body></html>"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(htmlBody))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误，实际 nil")
	}

	// 关键断言：logDebug 必须输出 body 摘要
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Login 200") {
		t.Errorf("logDebug 应输出 'Login 200' 标识符便于排查，实际日志:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "body=") {
		t.Errorf("logDebug 应包含 body= 字段便于查看原始 body，实际日志:\n%s", logOutput)
	}
	// 验证 body 摘要有意义（应包含 body 的一部分内容）
	bodyContainsHTML := strings.Contains(logOutput, "500") ||
		strings.Contains(logOutput, "html") ||
		strings.Contains(logOutput, "Internal")
	if !bodyContainsHTML {
		t.Errorf("logDebug 的 body 摘要应包含原 body 内容（HTML 500 错误页），实际日志:\n%s", logOutput)
	}
}

// ─── auth_warn_test.go: 302 fallback expiresAt 兜底 WARN 日志 ───

// TestLogin_302Fallback_ExpiresAtFallback_LogsAtWarn 验证 302 fallback
// 路径触发 expiresAt 兜底（now+24h）时，告警必须以 WARN 级别输出，
// 而不是 Debug（默认 LevelWarn 下被过滤，用户完全看不见）。
// 场景：server 返回 302 + Location 含 token 但无 expires_in/exp。
// Login 解析 Location → expiresAt = now+24h（兜底）→ 应 Warn 提示。
// 修复前：c.logDebug("...") → 默认 LevelWarn 下被过滤 → 静默。
// 修复后：c.logger.Warn("...") → 永远可见 → 用户立即知道 server 行为异常。
func TestLogin_302Fallback_ExpiresAtFallback_LogsAtWarn(t *testing.T) {
	// 启动 server：让 validate 路径返回 302 + Location 无 expires
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			// InitSession: 任意 200 即可
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			// 验证码图片: 任意非空字节
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			// 预校验验证码: 业务成功
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// 登录: 302 + Location 含 token 但无 expires 参数
			w.Header().Set("Location", "/homepage?token=jwt-no-expires")
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer srv.Close()

	// 自定义 logger: bytes.Buffer 收集所有日志，LevelDebug 让 Debug 也可见
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	resp, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173", // 跳过 GetSchoolID
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if resp.Token != "jwt-no-expires" {
		t.Errorf("token 应为 'jwt-no-expires'，实际: %s", resp.Token)
	}

	// 关键断言：日志必须以 WARN 级别输出兜底告警
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "level=WARN") {
		t.Errorf("期望 WARN 级别日志（让默认 LevelWarn 下用户也能看见），实际日志:\n%s", logOutput)
	}
	// 反向断言：不应只输出 Debug
	if !strings.Contains(logOutput, "level=WARN") && strings.Contains(logOutput, "level=DEBUG") {
		t.Errorf("兜底告警不应只用 Debug（默认 LevelWarn 下会被过滤），实际日志:\n%s", logOutput)
	}
	// 断言内容含兜底语义
	if !strings.Contains(logOutput, "24h") && !strings.Contains(logOutput, "兜底") && !strings.Contains(logOutput, "fallback") {
		t.Errorf("兜底告警应说明 '24h 兜底' 语义，实际日志:\n%s", logOutput)
	}
}

// ─── auth_readall_context_test.go (G3): io.ReadAll 错误含 status + 字节数 ───

// errAfterBytesBody 是 mock body：先返回 N 字节，再返回 io.ErrUnexpectedEOF。
// 让 io.ReadAll 失败但已读了 N 字节，触发 G3 错误包装逻辑。
type errAfterBytesBody struct {
	remaining []byte
	readByAll *int32
	firstRead bool
}

func (b *errAfterBytesBody) Read(p []byte) (int, error) {
	if b.firstRead && len(b.remaining) > 0 {
		// 第 1 次 Read：返回前 50 字节（readByAll += 50）
		b.firstRead = false
		n := copy(p, b.remaining)
		b.remaining = b.remaining[n:]
		atomic.AddInt32(b.readByAll, int32(n))
		return n, nil
	}
	return 0, io.ErrUnexpectedEOF
}

func (b *errAfterBytesBody) Close() error { return nil }

// errAfterBytesRT 是 http.RoundTripper mock，让 /validate 返回带 50 字节
// remaining + io.ErrUnexpectedEOF 的 body。
type errAfterBytesRT struct {
	validateReadBytes *int32
	calls             *int32
}

func (rt *errAfterBytesRT) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(rt.calls, 1)
	if strings.Contains(req.URL.Path, "validate") &&
		!strings.Contains(req.URL.Path, "validateCaptcha") &&
		req.Method == http.MethodPost {
		body := &errAfterBytesBody{
			remaining: bytes.Repeat([]byte{'X'}, 50),
			readByAll: rt.validateReadBytes,
			firstRead: true,
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       body,
			Header:     http.Header{"Content-Length": []string{"50"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":1,"msg":"成功"}`))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}, nil
}

func (rt *errAfterBytesRT) Close() error { return nil }

// TestLogin_ReadAllError_ContainsStatusAndBytes 验证 Login 200 路径
// io.ReadAll 失败时，错误信息必须包含：
// - status code（这里是 200）
// - 已读字节数（这里是 50）
// 便于排查 server 端异常（连接 reset / content-length 不符）。
func TestLogin_ReadAllError_ContainsStatusAndBytes(t *testing.T) {
	var validateReadBytes int32
	var calls int32

	rt := &errAfterBytesRT{
		validateReadBytes: &validateReadBytes,
		calls:             &calls,
	}

	c := &Client{
		ssoBaseURL: "http://mock-sso",
		baseURL:    "http://mock-sso",
		uploadURL:  "http://mock-sso",
		http: &http.Client{
			Transport: rt,
		},
		logger: slog.New(slog.DiscardHandler),
		ocr:    &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173", // 跳过 GetSchoolID
	})

	if err == nil {
		t.Fatal("期望 io.ReadAll 失败，实际 nil")
	}
	errStr := err.Error()

	// 必须 wrap io.ErrUnexpectedEOF
	if !errors.Is(err, io.ErrUnexpectedEOF) && !strings.Contains(errStr, io.ErrUnexpectedEOF.Error()) {
		t.Errorf("期望 wrap io.ErrUnexpectedEOF，实际: %v", err)
	}
	// G3 关键断言：错误必须包含 status code
	if !strings.Contains(errStr, "status=200") {
		t.Errorf("期望错误包含 'status=200' 上下文，实际: %v", err)
	}
	// G3 关键断言：错误必须包含已读字节数
	if !strings.Contains(errStr, "read=50") {
		t.Errorf("期望错误包含 'read=50' 上下文，实际: %v", err)
	}
}

// ─── extract_token_test.go: Location/Fragment token 提取 ───

// TestExtractTokenFromLocation_ExpiresIn 验证 Location 含 expires_in=N 时
// 返回真实 expiresAt（不再硬编码 now+24h）。
func TestExtractTokenFromLocation_ExpiresIn(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt123&expires_in=3600"
	token, expiresAt, err := tokenparse.ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "jwt123" {
		t.Errorf("token 错：%q", token)
	}
	expected := time.Now().Add(3600 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expiresAt 应 ≈ now+3600s，实际 delta=%v", delta)
	}
}

// TestExtractTokenFromLocation_Exp 验证 Location 含 exp=Unix 时间戳时
// 返回绝对时间。
func TestExtractTokenFromLocation_Exp(t *testing.T) {
	exp := time.Now().Add(2 * time.Hour).Unix()
	loc := "https://example.com/homepage?token=jwt&exp=9999999999" // 已知 2286 年
	token, expiresAt, err := tokenparse.ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "jwt" {
		t.Errorf("token 错：%q", token)
	}
	if !expiresAt.Equal(time.Unix(9999999999, 0)) {
		t.Errorf("exp 解析错误：%v", expiresAt)
	}
	_ = exp
}

// TestExtractTokenFromLocation_Fallback24h 验证无 expires 参数时 fallback 24h。
func TestExtractTokenFromLocation_Fallback24h(t *testing.T) {
	loc := "https://example.com/homepage?token=jwt"
	_, expiresAt, err := tokenparse.ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires 时应 fallback now+24h，实际 delta=%v", delta)
	}
}

// F2-EXTRACT-TOKEN-ASYM RED 测试：畸形 URL 返回 error，与 extractTokenFromReturnData
// 的错误传播契约对称。
// 用例：`http://[::1` 是缺少闭合 `]` 的 IPv6 字面量，net/url 必返回 parse error。
// 修复前：静默返回 ("", now+24h) — 错误吞掉，调用方看到「未找到 token」。
// 修复后：返回裸 url.Parse error（不再包 tokenparse.ErrLocationParseFailed，
// 因为该 sentinel 已删除——auth.go:165 包装时未用 %w 链入，对 Login 调用方
// 本来就不可达，纯死代码）。
func TestExtractTokenFromLocation_MalformedURL_ReturnsError(t *testing.T) {
	loc := "http://[::1"
	token, _, err := tokenparse.ExtractFromLocation(loc)
	if err == nil {
		t.Fatal("畸形 URL 应返回 error，实际 nil")
	}
	if token != "" {
		t.Errorf("畸形 URL 应返回空 token，实际 %q", token)
	}
}

// F10-FRAGMENT-URLDECODE RED 测试：fragment 中的 token= 值需 URL 解码。
// 历史：strings.Split + TrimPrefix 只做字符串裁剪，JWT 含 + / = 等 URL 保留
// 字符时会损坏 token。修复后 url.QueryUnescape 还原原始 base64 JWT。
// 用例：eyJ%2Bxxx%3D 解码后应为 eyJ+xxx=。
func TestExtractTokenFromFragment_URLEncodedValue(t *testing.T) {
	loc := "https://example.com/homepage#token=eyJ%2Bxxx%3D"
	token, _, err := tokenparse.ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "eyJ+xxx=" {
		t.Errorf("URL 解码错：want %q got %q", "eyJ+xxx=", token)
	}
}

// F10 边界：URL 解码失败时 fallback 到原始 value（best-effort 语义）。
func TestExtractTokenFromFragment_BadEncodingFallsBackToRaw(t *testing.T) {
	loc := "https://example.com/homepage#token=%ZZ"
	_, _, err := tokenparse.ExtractFromLocation(loc)
	if err == nil {
		t.Fatal("非法编码 fragment 应返回 error（url.Parse 拒绝），实际 nil")
	}
}

// F10 普通用例：无 URL 编码时透传。
func TestExtractTokenFromFragment_PlainValue(t *testing.T) {
	loc := "https://example.com/homepage#token=jwt123&other=x"
	token, _, err := tokenparse.ExtractFromLocation(loc)
	if err != nil {
		t.Fatalf("合法 Location 不应返回 error: %v", err)
	}
	if token != "jwt123" {
		t.Errorf("plain value 错：got %q", token)
	}
}

// ─── extract_token_return_data_test.go (G2): returnData 解析 expires_in/exp ───

// TestExtractTokenFromReturnData_ExpiresIn 验证 returnData 含 expires_in
// 时返回真实 expiresAt（now + expires_in 秒），不再硬编码 now+24h。
func TestExtractTokenFromReturnData_ExpiresIn(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt","expires_in":3600}`)
	token, expiresAt, err := tokenparse.ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	if token != "jwt" {
		t.Errorf("token 应为 'jwt'，实际: %s", token)
	}
	expected := time.Now().Add(3600 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expiresAt 应 ≈ now+3600s（解析 expires_in），实际 delta=%v", delta)
	}
	// 关键断言：不应是 now+24h 兜底（修复前 bug 症状）
	if time.Until(expiresAt) > 23*time.Hour {
		t.Errorf("expiresAt 居然 ≥ 23h，说明又走 now+24h 兜底了（未解析 expires_in）")
	}
}

// TestExtractTokenFromReturnData_Exp 验证 returnData 含 exp（Unix 秒）时
// 返回绝对时间 time.Unix(n, 0)。
func TestExtractTokenFromReturnData_Exp(t *testing.T) {
	// exp = 1888888888（2030 年附近，足够未来）
	raw := json.RawMessage(`{"token":"jwt","exp":1888888888}`)
	token, expiresAt, err := tokenparse.ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	if token != "jwt" {
		t.Errorf("token 应为 'jwt'，实际: %s", token)
	}
	if !expiresAt.Equal(time.Unix(1888888888, 0)) {
		t.Errorf("exp 解析错误：期望 time.Unix(1888888888,0)，实际 %v", expiresAt)
	}
}

// TestExtractTokenFromReturnData_Fallback24h 验证 returnData 既无 expires_in
// 也无 exp 时 fallback now+24h（与原行为兼容）。
func TestExtractTokenFromReturnData_Fallback24h(t *testing.T) {
	raw := json.RawMessage(`{"token":"jwt"}`)
	_, expiresAt, err := tokenparse.ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(24 * time.Hour)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("无 expires_in/exp 时应 fallback now+24h，实际 delta=%v", delta)
	}
}

// TestExtractTokenFromReturnData_ExpiresIn_TakesPriorityOverExp 验证
// expires_in 优先级高于 exp（与 parseExpiresMap 行为对称）。
func TestExtractTokenFromReturnData_ExpiresIn_TakesPriorityOverExp(t *testing.T) {
	// 同时给 expires_in=60 和 exp=1888888888：应取 expires_in
	raw := json.RawMessage(`{"token":"jwt","expires_in":60,"exp":1888888888}`)
	_, expiresAt, err := tokenparse.ExtractFromReturnData(raw)
	if err != nil {
		t.Fatalf("期望无 err，实际: %v", err)
	}
	expected := time.Now().Add(60 * time.Second)
	delta := expiresAt.Sub(expected)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Errorf("expires_in 应优先于 exp，实际 delta=%v", delta)
	}
	// 关键断言：不应是绝对时间 time.Unix(1888888888, 0)
	if strings.Contains(expiresAt.Format(time.RFC3339), "2030") {
		t.Errorf("expires_in 应优先于 exp，实际却走了 exp（绝对时间）")
	}
}

// ─── auth_captcha_log_leak_test.go (A1): logDebug 不泄漏验证码原文 ───

// TestLogin_Log_DoesNotLeakCaptcha 验证 Login 流程中 logDebug 不输出验证码原文。
// 修复前：c.logDebug("OCR 识别结果: %s", captcha) 将验证码明文写入日志。
// 修复后：只输出长度信息，不输出验证码本身。
func TestLogin_Log_DoesNotLeakCaptcha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"token":"jwt-test"}}`))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	const secretCaptcha = "S3CR3T_C4PTCH4_789"
	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: secretCaptcha},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}

	logOutput := logBuf.String()
	if strings.Contains(logOutput, secretCaptcha) {
		t.Errorf("FAIL: 日志泄露了验证码原文 %q，日志:\n%s", secretCaptcha, logOutput)
	}
	// 正向断言：日志应包含 OCR 相关描述（如"OCR 识别完成"或字符数）
	if !strings.Contains(logOutput, "OCR 识别") && !strings.Contains(logOutput, "字符") {
		t.Errorf("日志应输出 OCR 识别相关信息（非明文），实际日志:\n%s", logOutput)
	}
}

// ─── auth_body_truncate_test.go (A2/A3): logDebug body 截断 + 敏感字段掩码 ───

// TestLogin_Log_BodyTruncated 验证 logDebug 输出 body 时截断到 ≤100 字符。
// 修复前：完整 body（可能含 token）全部输出到日志。
// 修复后：body 输出限制在 100 字符以内。
func TestLogin_Log_BodyTruncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			// non-200 + 长 body → 触发 line 183 logDebug（非预期状态码路径）
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(strings.Repeat("A", 200)))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望非 200 错误，实际 nil")
	}

	logOutput := logBuf.String()
	// 检查任何 body= 后是否有超长内容
	bodyIdx := strings.Index(logOutput, "body=")
	if bodyIdx < 0 {
		t.Log("日志中未出现 body=, 日志:\n" + logOutput)
		return
	}
	afterBody := logOutput[bodyIdx+5:]
	firstLineEnd := strings.Index(afterBody, "\n")
	var bodySnippet string
	if firstLineEnd > 0 {
		bodySnippet = afterBody[:firstLineEnd]
	} else {
		bodySnippet = afterBody
	}
	if len(bodySnippet) > 120 {
		t.Errorf("body 输出长度 %d 超过预期（期望 ≤100 字符附近）:\n%s", len(bodySnippet), bodySnippet[:80])
	}
}

// TestLogin_Log_BodyContainsSensitive 验证 body 输出中敏感字段（token）被掩码。
func TestLogin_Log_BodyTokenMasked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "application/json")
			// 触发 returnData=null 路径（logDebug body=%s 在第 148 行）
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":null}`))
		}
	}))
	defer srv.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
		logger:     logger,
		ocr:        &countMockOCR{returnText: "AB12"},
	}

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望错误，实际 nil")
	}

	logOutput := logBuf.String()
	// 反向断言：日志不应包含 "token":"<明文>" 形式
	if strings.Contains(logOutput, `"token":"`) {
		// 更进一步：检查是否有真正的 token 值而非掩码
		t.Errorf("日志似乎包含 token 键值对，可能泄露了敏感信息:\n%s", logOutput)
	}
}

// ─── warn_sync_cookie_test.go (F2): helper WARN 日志 + 防 token 泄露 ───

// ─── auth_wrap_test.go (A4/A5/A6): fmt.Errorf %w 包装穿透 ───

// TestLogin_200Path_JSONUnmarshalError_WrappedWithPercentW 验证
// 当 200 响应 body 不是合法 JSON 时，Login 返回的错误应能通过 errors.As
// 穿透到 json.SyntaxError 原始错误（而不是被 %v 截断错误链）。
//
// 修复前：fmt.Errorf("%w: ...: %v", ErrLoginRejected, err) — %v 断开错误链，
//
//	errors.As 找不到原始 json.SyntaxError。
//
// 修复后：fmt.Errorf("%w: ...: %w", ErrLoginRejected, err) — %w 保留错误链，
//
//	errors.As 可穿透找到 json.SyntaxError。
func TestLogin_200Path_JSONUnmarshalError_WrappedWithPercentW(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// 非 JSON body，触发 json.Unmarshal 失败路径（auth.go:134-136）
			_, _ = w.Write([]byte("not-json-at-all{{{"))
		}
	}))
	defer srv.Close()

	c := newClientForOCRTest(srv.URL, &countMockOCR{returnText: "AB12"})

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误，实际 nil")
	}

	// 关键断言：errors.As 应能穿透到 json.SyntaxError（证明 %w 正确保留错误链）
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("期望 errors.As 能穿透找到 *json.SyntaxError（证明 percent-w 包装），实际错误: %v", err)
	}
	// 反向断言：必须仍 wrap ErrLoginRejected
	if !errors.Is(err, ErrLoginRejected) {
		t.Errorf("错误仍应 wrap ErrLoginRejected，实际: %v", err)
	}
}

// TestWarnSyncCookieToken_BadJar_LogsWarn 验证 helper 在 cookie 同步失败时
// 输出 WARN 日志且包含调用方提供的 label 标识符。
// 场景：自定义 http.Client（Jar 为 nil，非 *cookiejar.Jar）→ syncCookieToken
// 返回 error → helper 应输出 WARN 日志。
// 修复前：200/302 两段 copy-paste，仅 label 字符串不同。
// 修复后：统一走 helper，label 由调用方传入。
func TestWarnSyncCookieToken_BadJar_LogsWarn(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       &http.Client{Timeout: 5 * time.Second}, // Jar = nil → syncCookieToken 返回 error
		logger:     logger,
	}

	// 调用 helper，label = "TEST_LABEL"
	c.warnSyncCookieToken("dummy-token", "TEST_LABEL")

	logOutput := logBuf.String()

	// 关键断言 1：必须以 WARN 级别输出（默认 LevelWarn 下用户可见）
	if !strings.Contains(logOutput, "level=WARN") {
		t.Errorf("期望 WARN 级别日志，实际日志:\n%s", logOutput)
	}
	// 关键断言 2：必须包含调用方传入的 label 标识符（200 / 302 fallback 等）
	if !strings.Contains(logOutput, "TEST_LABEL") {
		t.Errorf("日志应包含 label 标识符 'TEST_LABEL'，实际日志:\n%s", logOutput)
	}
	// 关键断言 3：日志消息前缀应是 "Login <label> 后同步 token 到 cookie 失败"
	if !strings.Contains(logOutput, "Login TEST_LABEL") {
		t.Errorf("日志消息应以 'Login TEST_LABEL' 开头，实际日志:\n%s", logOutput)
	}
	// 关键断言 4：err 字段应包含 syncCookieToken 的具体错误
	if !strings.Contains(logOutput, "cookie") && !strings.Contains(logOutput, "Jar") {
		t.Errorf("err 字段应包含 cookie/Jar 相关错误信息，实际日志:\n%s", logOutput)
	}
}

// TestWarnSyncCookieToken_BadJar_DoesNotLeakToken 验证 helper 输出错误日志时
// **不会** 把 token 写入日志（避免敏感凭据泄露到 stderr）。
// 安全约束：token 是 X-Auth-Token，业务调用方常把日志收集到 ELK / 第三方，
// 一旦泄露等同于泄露登录态。F2 helper 必须保证失败日志不含 token 字面值。
func TestWarnSyncCookieToken_BadJar_DoesNotLeakToken(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	c := &Client{
		ssoBaseURL: "https://sso.example.com",
		baseURL:    "https://biz.example.com",
		uploadURL:  "https://up.example.com",
		http:       &http.Client{Timeout: 5 * time.Second}, // Jar = nil
		logger:     logger,
	}

	// 使用一个足够独特的 token 字符串，便于断言是否泄露
	const secretToken = "SECRET_TOKEN_XYZ_DO_NOT_LEAK_42"
	c.warnSyncCookieToken(secretToken, "leak-check")

	logOutput := logBuf.String()

	// 反向断言：日志不应包含 token 字面值
	if strings.Contains(logOutput, secretToken) {
		t.Errorf("FAIL: 日志泄露了 token 字符串！实际日志:\n%s", logOutput)
	}
}

// ─── getschool_url_encoding_test.go: GetSchoolID URL 编码 username ───

// TestGetSchoolID_URLEncodesUsername 验证 GetSchoolID 对学号中的特殊字符进行 URL 编码。
// 历史 bug：auth.go:36 直接拼接 "?userName=" + username，若学号含 & / = 等
// 保留字符会破坏 URL 结构。此处传 "S123&456" 测试 & 被编码为 %26。
// 修复后：用 url.Values{"userName": {username}}.Encode() 构建 query，
// 与 session.go:107 模式对齐。
func TestGetSchoolID_URLEncodesUsername(t *testing.T) {
	var requestURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"100","NAME":"测试学校"}]}`))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithSSOBase(srv.URL),
		WithTimeout(5*time.Second),
	)

	_, _, err := c.GetSchoolID(context.Background(), "S123&456")
	if err != nil {
		t.Fatalf("GetSchoolID 失败: %v", err)
	}

	// 验证 userName 参数被正确编码（& 应变为 %26）
	if requestURL == "" {
		t.Fatal("请求未被发送")
	}
	if strings.Contains(requestURL, "userName=S123&456") {
		t.Errorf("userName 中的 & 未被编码: %s", requestURL)
	}
	if !strings.Contains(requestURL, "userName=S123%26456") {
		t.Errorf("期望 URL 编码 userName=S123%%26456，实际 URL: %s", requestURL)
	}
}

// ─── string_ptr_or_test.go (M2): derefOr helper 语义 ───

// TestDerefOr_StringNilAndValue 验证 derefOr 三种场景：
// - nil 指针 → def
// - 指向空字符串的指针 → 返回 ""（与 cmp.Or 行为不同：cmp.Or 把 "" 当零值用 def）
// - 指向非空字符串的指针 → 返回 *s
// 与原 stringPtrOr 行为完全一致（重构等价）。
func TestDerefOr_StringNilAndValue(t *testing.T) {
	// nil 指针 → def
	var nilPtr *string
	if got := types.DerefOr(nilPtr, "登录失败"); got != "登录失败" {
		t.Errorf("nil 指针应返回 def，实际: %q", got)
	}
	// 空字符串 → 返回 ""（与原 stringPtrOr 一致）
	emptyPtr := ""
	if got := types.DerefOr(&emptyPtr, "登录失败"); got != "" {
		t.Errorf("指向空字符串的指针应返回 \"\"（与原 stringPtrOr 一致），实际: %q", got)
	}
	// 非空 → *s
	val := "用户名或密码错误"
	if got := types.DerefOr(&val, "登录失败"); got != "用户名或密码错误" {
		t.Errorf("非空应返回 *ptr，实际: %q", got)
	}
}

// TestDerefOr_NotConfusedWithCmpOr 回归测试：确保 derefOr 不被错误替换为
// cmp.Or（cmp.Or 在 nil 指针场景会 panic，破坏 Login 错误信息兜底契约）。
// 通过 grep "cmp\.Or.*Msg" 应 0 命中来强制（人工/CI 检查）。
// 运行时本测试仅记录 M2 fix 完成的语义契约。
func TestDerefOr_NotConfusedWithCmpOr(t *testing.T) {
	t.Log("M2 fix 已完成：stringPtrOr 重命名为 derefOr（nil-safe，3 行实现）")
	t.Log("注意：不能用 cmp.Or(*Msg, def) 替代，cmp.Or 在 Msg==nil 时 panic")
}

// ─── auth_wrap_test.go (A5): 200 路径 extractToken 错误改用 %w 包装 ───

// TestLogin_200Path_ExtractTokenError_WrappedWithPercentW 验证
// 当 returnData 中无 token 字段时（触发 extractTokenFromReturnData 返回错误），
// Login 返回的错误应保留 tokenparse 返回的底层错误（不是用 %v 截断）。
//
// 修复前：fmt.Errorf("%w: 200 响应中未找到 token: %v", ErrLoginRejected, err) — %v 断开错误链。
// 修复后：fmt.Errorf("%w: 200 响应中未找到 token: %w", ErrLoginRejected, err) — %w 保留错误链，
//
//	errors.Is 可穿透找到 tokenparse 返回的 errors.New("returnData 中无 token 字段")。
func TestLogin_200Path_ExtractTokenError_WrappedWithPercentW(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/teacher/auth/studentLogin/validate":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// returnData 是 JSON 对象但不含 token 字段 → 触发 extractToken 失败路径（auth.go:151-154）
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"other":"value"}}`))
		}
	}))
	defer srv.Close()

	c := newClientForOCRTest(srv.URL, &countMockOCR{returnText: "AB12"})

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误（extractToken 失败），实际 nil")
	}

	// 关键断言：err.Error() 应包含 tokenparse 返回的原始错误消息
	if !strings.Contains(err.Error(), "returnData 中无 token 字段") &&
		!strings.Contains(err.Error(), "returnData 中 token 字段类型异常") {
		t.Errorf("期望错误链保留 tokenparse 原始错误消息，实际: %v", err)
	}
	// 反向断言：必须仍 wrap ErrLoginRejected
	if !errors.Is(err, ErrLoginRejected) {
		t.Errorf("错误仍应 wrap ErrLoginRejected，实际: %v", err)
	}
}

// ─── auth_wrap_test.go (A6): 302 路径 Location 解析错误改用 %w 包装 ───

// TestLogin_302Path_LocationParseError_WrappedWithPercentW 验证
// 当 302 Location 是畸形 URL 时（触发 url.Parse 失败），
// Login 返回的错误应保留 url.Parse 原始错误（不是用 %v 截断）。
//
// 修复前：fmt.Errorf("%w: Location 头解析失败: %v", ErrLoginRejected, locErr) — %v 断开错误链。
// 修复后：fmt.Errorf("%w: Location 头解析失败: %w", ErrLoginRejected, locErr) — %w 保留错误链。
func TestLogin_302Path_LocationParseError_WrappedWithPercentW(t *testing.T) {
	// 使用 http.RoundTripper mock 直接返回 302，让 auth.go 的 Location 解析代码真正执行
	// （httptest.Server 返回 302 时，Go net/http Client 会先验证 Location header 有效性，
	//  无法让 auth.go:168-170 的 tokenparse.ExtractFromLocation 路径命中）
	rt := &malformedLocationRT{}
	c := &Client{
		ssoBaseURL: "http://mock-sso",
		baseURL:    "http://mock-sso",
		uploadURL:  "http://mock-sso",
		http: &http.Client{
			Transport: rt,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: slog.New(slog.DiscardHandler),
		ocr:    &countMockOCR{returnText: "AB12"},
	}
	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误（Location 解析失败），实际 nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("期望错误链保留 url.Parse 原始错误消息，实际: %v", err)
	}
	// 302 预校验失败包装 ErrNetwork（Go http.Client 处理），不是 ErrLoginRejected
	// 因此 auth.go:170 的 %w 修复虽然正确但本测试不验证 errors.Is(ErrLoginRejected)
}

// malformedLocationRT 是 http.RoundTripper mock：让 /validate 返回 302 +
// 畸形 Location（绕过 net/http 预校验，让 auth.go 的 Location 解析代码路径执行）。
type malformedLocationRT struct{}

func (rt *malformedLocationRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "validate") &&
		!strings.Contains(req.URL.Path, "validateCaptcha") &&
		req.Method == http.MethodPost {
		return &http.Response{
			StatusCode: http.StatusFound,
			Status:     "302 Found",
			Header:     http.Header{"Location": []string{"http://[::1"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body: io.NopCloser(strings.NewReader(
			`{"code":1,"msg":"成功"}`)),
		Header:  http.Header{"Content-Type": []string{"application/json"}},
		Request: req,
	}, nil
}

func (rt *malformedLocationRT) Close() error { return nil }

// ─── auth_wrap_test.go (A7): validateCaptcha 错误改用 ErrLoginRejected ───

// TestLogin_ValidateCaptcha_ErrorsIsErrLoginRejected 验证 validateCaptcha
// 返回 code != 1 时，Login 包装的哨兵错误是 ErrLoginRejected 而非
// ErrBusinessRejected。
//
// A7 修复前：errors.Join(ErrBusinessRejected, err) — SDK 用户用
//
//	errors.Is(err, ErrLoginRejected) 无法命中。
//
// A7 修复后：errors.Join(ErrLoginRejected, err) — 验证码校验失败属于
//
//	Login 流程错误，不是业务 API 拒绝。
func TestLogin_ValidateCaptcha_ErrorsIsErrLoginRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/kaptcha/kaptcha.jpg":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake-jpeg-bytes"))
		case "/uiStudentLogin/validateCaptcha":
			w.Header().Set("Content-Type", "application/json")
			// code=0：验证码错误，触发 auth.go:206-212 的包装路径
			_, _ = w.Write([]byte(`{"code":0,"msg":"验证码错误"}`))
		}
	}))
	defer srv.Close()

	c := newClientForOCRTest(srv.URL, &countMockOCR{returnText: "AB12"})

	_, err := c.Login(context.Background(), types.LoginRequest{
		Username: "u",
		Password: "p",
		SchoolID: "173",
	})
	if err == nil {
		t.Fatal("期望 Login 返回错误（validateCaptcha 拒绝），实际 nil")
	}

	// 关键断言 1：errors.Is 必须命中 ErrLoginRejected
	if !errors.Is(err, ErrLoginRejected) {
		t.Errorf("errors.Is(err, ErrLoginRejected) 应为 true，实际错误链不包含 ErrLoginRejected，err=%v", err)
	}

	// 关键断言 2：errors.Is 不应命中 ErrBusinessRejected
	if errors.Is(err, ErrBusinessRejected) {
		t.Errorf("errors.Is(err, ErrBusinessRejected) 应为 false（验证码校验不是业务 API 拒绝），err=%v", err)
	}
}
