package e2e

import (
	"bytes"
	"context"
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
			// 登录走 SDK 默认内置的 nazhi-captcha-sdk 本地验证码识别器，零配置。
			cLogin, err3 := client.New(
				client.WithSSOBase(ssoBase),
				client.WithBaseURL(bizBase),
				client.WithUploadURL(uploadBase),
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
