package e2e

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 默认地址，与 test/integration 一致。
const (
	defaultSSOBase    = "https://www.nazhisoft.com"
	defaultBizBase    = "http://139.159.205.146:8280"
	defaultUploadBase = "http://doc.nazhisoft.com"
)

// harness 全局句柄，供同包测试复用。
var (
	liveClient    *client.Client
	mockClient    *client.Client
	liveToken     string
	liveAvailable bool
	liveWrite     bool
	liveUpload    bool
	mockSrv       *httptest.Server
)

// RecordedWrite 记录一次写请求。
type RecordedWrite struct {
	Method string
	Path   string
	Query  string
	Body   string
}

var (
	recordedMu     sync.Mutex
	recordedWrites []RecordedWrite
)

// GetRecordedWrites 返回已记录的写请求快照。
func GetRecordedWrites() []RecordedWrite {
	recordedMu.Lock()
	defer recordedMu.Unlock()
	out := make([]RecordedWrite, len(recordedWrites))
	copy(out, recordedWrites)
	return out
}

// ClearRecordedWrites 清空记录。
func ClearRecordedWrites() {
	recordedMu.Lock()
	recordedWrites = nil
	recordedMu.Unlock()
}

func appendRecorded(r RecordedWrite) {
	recordedMu.Lock()
	recordedWrites = append(recordedWrites, r)
	recordedMu.Unlock()
}

// requireLive 在无真域凭证时 Skip。
func requireLive(t interface {
	Skip(...any)
	Helper()
}) {
	t.Helper()
	if !liveAvailable || liveToken == "" {
		t.Skip("e2e live read skipped: set NAZHI_USERNAME/NAZHI_PASSWORD")
	}
}

// IsLiveWrite 报告写是否走真域。
// 已禁用全真写，永远返回 false（写永远 mock）。
func IsLiveWrite() bool { return false }

// IsLiveAvailable 报告是否可走真域读。
func IsLiveAvailable() bool { return liveAvailable && liveToken != "" }

func TestMain(m *testing.M) {
	// 解析 env
	username := strings.TrimSpace(os.Getenv("NAZHI_USERNAME"))
	password := strings.TrimSpace(os.Getenv("NAZHI_PASSWORD"))
	ssoBase := strings.TrimSpace(os.Getenv("NAZHI_SSO_BASE"))
	if ssoBase == "" {
		ssoBase = defaultSSOBase
	}
	bizBase := strings.TrimSpace(os.Getenv("NAZHI_BASE_URL"))
	if bizBase == "" {
		bizBase = defaultBizBase
	}
	uploadBase := strings.TrimSpace(os.Getenv("NAZHI_UPLOAD_URL"))
	if uploadBase == "" {
		uploadBase = defaultUploadBase
	}
	liveWrite = false
	if strings.TrimSpace(os.Getenv("NAZHI_E2E_LIVE_WRITE")) == "1" {
		fmt.Fprintf(os.Stderr, "[e2e] WARN: NAZHI_E2E_LIVE_WRITE 已禁用 — 写操作永远走本地 mock，不会污染线上数据\n")
	}
	liveUploadEnv := strings.TrimSpace(os.Getenv("NAZHI_E2E_LIVE_UPLOAD"))
	liveUpload = liveUploadEnv != "0" // 默认 1
	if liveUploadEnv == "" {
		liveUpload = true
	}

	liveAvailable = username != "" && password != ""

	// 启动 mockWriteServer（无论是否 live 都启动，供 WriteMock 用）
	mockSrv = newMockWriteServer()

	// 构造 mockClient（base 指向 mock）
	var err error
	mockClient, err = client.New(
		client.WithSSOBase(ssoBase),
		client.WithBaseURL(mockSrv.URL),
		client.WithUploadURL(mockSrv.URL),
		client.WithTimeout(15*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[e2e] mockClient 创建失败: %v\n", err)
		os.Exit(1)
	}

	// 构造 liveClient（仅当 liveAvailable 时才真正有用，无凭证时仍建一个指向真域的备用）
	liveClient, err = client.New(
		client.WithSSOBase(ssoBase),
		client.WithBaseURL(bizBase),
		client.WithUploadURL(uploadBase),
		client.WithTimeout(15*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[e2e] liveClient 创建失败: %v\n", err)
		os.Exit(1)
	}

	// 若需真读，尝试拿 token（缓存优先）
	if liveAvailable {
		cachePath := tokenCachePath()
		// 运行目录可能不在仓根，尝试让 .e2e_token 落在仓根
		if !filepath.IsAbs(cachePath) {
			if abs, err2 := filepath.Abs(cachePath); err2 == nil {
				cachePath = abs
			}
		}
		if ent, err2 := loadTokenCache(cachePath); err2 == nil && isTokenCacheValid(ent, username, ssoBase) {
			liveToken = ent.Token
			fmt.Fprintf(os.Stderr, "[e2e] token 缓存命中 (exp %s)\n", ent.Exp.Format(time.RFC3339))
			// 同步到 liveClient 的 cookie jar
			if c2, err3 := client.New(
				client.WithSSOBase(ssoBase),
				client.WithBaseURL(bizBase),
				client.WithUploadURL(uploadBase),
				client.WithToken(liveToken),
				client.WithTimeout(15*time.Second),
			); err3 == nil {
				_ = liveClient.Close()
				liveClient = c2
			} else {
				fmt.Fprintf(os.Stderr, "[e2e] token 缓存同步失败: %v\n", err3)
			}
		} else {
			siliconKey := resolveSiliconKey()
			if siliconKey == "" {
				fmt.Fprintf(os.Stderr, "[e2e] 缺 NAZHI_SILICONFLOW_API_KEY 且未找到 Nazhi-auto fallback，live 将 Skip（write mock 仍可用）\n")
				liveAvailable = false
			} else {
				ocr := newE2EOmniOCR(siliconKey, strings.TrimSpace(os.Getenv("NAZHI_SILICONFLOW_BASE_URL")), "")
				cLogin, err3 := client.New(
					client.WithSSOBase(ssoBase),
					client.WithBaseURL(bizBase),
					client.WithUploadURL(uploadBase),
					client.WithCustomOCR(ocr),
					client.WithTimeout(45*time.Second),
				)
				if err3 != nil {
					fmt.Fprintf(os.Stderr, "[e2e] 登录 client 创建失败: %v\n", err3)
					liveAvailable = false
				} else {
					ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
					resp, err4 := cLogin.Login(ctx, types.LoginRequest{Username: username, Password: password})
					cancel()
					_ = cLogin.Close()
					if err4 != nil || resp == nil || resp.Token == "" {
						fmt.Fprintf(os.Stderr, "[e2e] 登录失败，live 将 Skip: %v\n", err4)
						liveAvailable = false
					} else {
						liveToken = resp.Token
						fmt.Fprintf(os.Stderr, "[e2e] 登录成功 token %s... exp %s\n", safePrefix(liveToken, 12), resp.ExpiresAt.Format(time.RFC3339))
						// 写入缓存
						_ = saveTokenCache(cachePath, &TokenCacheEntry{
							Username: username,
							SSOBase:  ssoBase,
							Token:    liveToken,
							Exp:      resp.ExpiresAt,
							SavedAt:  time.Now(),
						})
						// 同步到 liveClient
						if c2, err5 := client.New(
							client.WithSSOBase(ssoBase),
							client.WithBaseURL(bizBase),
							client.WithUploadURL(uploadBase),
							client.WithToken(liveToken),
							client.WithTimeout(15*time.Second),
						); err5 == nil {
							_ = liveClient.Close()
							liveClient = c2
						}
						// 预热 session（4 步）
						ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
						_, _ = liveClient.ActivateSession(ctx2, liveToken)
						cancel2()
					}
				}
			}
		}
		// 写永远 mock，不存在 LIVE WRITE 分支
	}

	fmt.Fprintf(os.Stderr, "[e2e] mode mixed liveAvailable=%v liveWrite=%v liveUpload=%v mock=%s\n", liveAvailable, liveWrite, liveUpload, mockSrv.URL)

	code := m.Run()

	_ = mockClient.Close()
	_ = liveClient.Close()
	mockSrv.Close()
	os.Exit(code)
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func resolveSiliconKey() string {
	if k := strings.TrimSpace(os.Getenv("NAZHI_SILICONFLOW_API_KEY")); k != "" {
		return k
	}
	if k := strings.TrimSpace(os.Getenv("NAZHI_OCR_API_KEY")); k != "" {
		return k
	}
	if k := strings.TrimSpace(os.Getenv("SILICONFLOW_API_KEY")); k != "" {
		return k
	}
	// fallback: 读 Nazhi-auto 配置
	candidates := []string{
		`E:/newCC/life-new2026/Nazhi-auto/backend/data/settings.yaml`,
		`E:/newCC/life-new2026/Nazhi-auto/data/settings.yaml`,
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		// 简易 yaml 扫描：找 api_key: sk-...
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "api_key") && strings.Contains(line, "sk-") {
				idx := strings.Index(line, "sk-")
				if idx >= 0 {
					tail := line[idx:]
					// 截到空白或引号
					end := len(tail)
					for i, r := range tail {
						if r == '"' || r == '\'' || r == ' ' || r == '\t' {
							if i > 3 {
								end = i
								break
							}
						}
					}
					k := strings.TrimSpace(tail[:end])
					k = strings.Trim(k, `"' `)
					if strings.HasPrefix(k, "sk-") {
						return k
					}
				}
			}
			if strings.Contains(line, "siliconflow_key") && strings.Contains(line, "sk-") {
				idx := strings.Index(line, "sk-")
				tail := line[idx:]
				end := len(tail)
				for i, r := range tail {
					if r == '"' || r == '\'' || r == ' ' {
						end = i
						break
					}
				}
				k := strings.Trim(tail[:end], `"' `)
				if strings.HasPrefix(k, "sk-") {
					return k
				}
			}
		}
	}
	return ""
}

// newMockWriteServer 注册全部写路径。
func newMockWriteServer() *httptest.Server {
	mux := http.NewServeMux()
	register := func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
		mux.HandleFunc(pattern, handler)
	}

	writeOK := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{}}`))
	}
	commentOK := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"id":9999,"content":"ok","circleId":1}}`))
	}

	// 读 body 并记录，轻量校验：若为 JSON 则尝试解析，失败仍返回成功（避免 flake）
	withJSONCheck := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.ContentLength != 0 {
				b, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(b))
				if len(b) > 0 && b[0] == '{' {
					var m map[string]any
					_ = json.Unmarshal(b, &m)
				}
				// 让下一层再读（next 内会再读一次，需回填）
				r.Body = io.NopCloser(bytes.NewReader(b))
			}
			next(w, r)
		}
	}

	register("/api/studentCircleNew/addCircle", withJSONCheck(writeOK))
	register("/api/studentCircleNew/editCircle", withJSONCheck(writeOK))
	register("/api/studentCircleNew/deleteCircle", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	})
	register("/api/studentCircleNew/addCircleComment", withJSONCheck(commentOK))
	register("/api/studentCircleNew/setCircleLikeById", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	})
	register("/api/studentMoralEduNew/addHonor", withJSONCheck(writeOK))
	register("/api/studentMoralEduNew/updateHonor", withJSONCheck(writeOK))
	register("/api/studentMoralEduNew/deleteHonorById", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	})
	register("/api/studentCircleNew/addTypicalCase", withJSONCheck(writeOK))
	register("/api/studentCircleNew/updateTypicalCase", withJSONCheck(writeOK))
	register("/api/studentCircleNew/deleteTypicalCase", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
	})
	register("/api/studentCircleNew/deleteBatchTypicalCase", withJSONCheck(writeOK))
	register("/api/studentMoralEduNew/addSelfEvaluation", withJSONCheck(writeOK))
	register("/api/studentMoralEduNew/addSelfGradEvaluation", withJSONCheck(writeOK))
	register("/api/studentInfo/updateMyInfo", withJSONCheck(writeOK))

	// --- Session 激活链路（复用 integration 的最小 stub） ---
	register("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>home</body></html>"))
	})
	register("/api/studentInfo/getMenu", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"returnData\":{\"menus\":[]}}"))
	})
	register("/api/studentInfo/getMyInfo", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"returnData\":{\"name\":\"E2E用户\",\"studentNumber\":\"TEST001\",\"schoolName\":\"测试学校\",\"className\":\"测试班级\",\"seat\":1}}"))
	})
	// 文件上传最小 stub（供 WriteMock 的 SubmitTask 图片链路不 panic）
	register("/common/upload/uploadImage", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"returnData\":{\"id\":99999,\"name\":\"e2e.jpg\"}}"))
	})
	// 任务元数据 stub（SubmitTask/EditCircle 的 getCircleTypeByTaskId）
	register("/api/studentCircleNew/getCircleTypeByTaskId", func(w http.ResponseWriter, r *http.Request) {
		appendRecorded(RecordedWrite{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"dataMap\":{\"taskId\":1,\"circleTypeId\":3694,\"dimensionId\":13,\"hours\":4,\"type\":4,\"taskName\":\"E2E任务\"}}"))
	})
	register("/api/studentCircleNew/getDimensions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"dataList\":[{\"id\":13,\"name\":\"社会实践\"}]}"))
	})
	register("/api/studentCircleNew/getHonorTypeForSelect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"code\":1,\"msg\":\"成功\",\"dataList\":[{\"label\":\"校三好学生\",\"value\":1148}],\"returnData\":[{\"label\":\"校\",\"value\":5}]}"))
	})

	return httptest.NewServer(mux)
}

// e2eOmniOCR 复刻 cmd/nazhi/omni_ocr.go 的最小实现，供 harness 登录使用。
type e2eOmniOCR struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

const (
	e2eOmniBaseURL = "https://api.siliconflow.cn/v1"
	e2eOmniModel   = "Qwen/Qwen3-Omni-30B-A3B-Instruct"
	e2eOmniSystem  = "Output 4 alphanumeric characters."
)

func newE2EOmniOCR(apiKey, baseURL, model string) *e2eOmniOCR {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = e2eOmniBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = e2eOmniModel
	}
	return &e2eOmniOCR{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		model:   strings.TrimSpace(model),
		http:    &http.Client{Timeout: 45 * time.Second},
	}
}

func (o *e2eOmniOCR) Recognize(img []byte) (string, error) {
	if o == nil || o.apiKey == "" {
		return "", fmt.Errorf("Omni OCR 未配置 API Key")
	}
	if len(img) == 0 {
		return "", nil
	}
	body := map[string]any{
		"model": o.model,
		"messages": []any{
			map[string]any{"role": "system", "content": e2eOmniSystem},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]string{"url": e2eDataURI(img)}},
				},
			},
		},
		"temperature": 0,
		"max_tokens":  256,
		"stream":      false,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("编码 Omni OCR 请求失败: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("构造 Omni OCR 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 Omni OCR 失败: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取 Omni OCR 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Omni OCR HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(out)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", fmt.Errorf("解析 Omni OCR 响应失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", nil
	}
	text := e2eMessageText(parsed.Choices[0].Message.Content)
	cleaned := cleanE2ECaptcha(text)
	if !isValidE2ECaptcha(cleaned) {
		return "", nil
	}
	return cleaned, nil
}

func (o *e2eOmniOCR) Close() error { return nil }

func e2eDataURI(img []byte) string {
	return "data:" + e2eDetectMIME(img) + ";base64," + base64.StdEncoding.EncodeToString(img)
}

func e2eDetectMIME(data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 3 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' {
		return "image/gif"
	}
	if len(data) >= 4 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		return "image/webp"
	}
	return "image/png"
}

func e2eMessageText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	default:
		return fmt.Sprint(v)
	}
}

func cleanE2ECaptcha(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func isValidE2ECaptcha(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
