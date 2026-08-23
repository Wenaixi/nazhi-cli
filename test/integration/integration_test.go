// Package integration 包含需要真实 SSO/业务服务器环境的集成测试。
//
// 通过 build tag `integration` 启用：
//
//	NAZHI_USERNAME=学号 NAZHI_PASSWORD=密码 go test -tags=integration -v ./test/integration/...
//
// 或通过 .env 文件：
//
//	make test-integration
//
// 若 NAZHI_USERNAME / NAZHI_PASSWORD 未设置，测试自动 t.Skip 跳过。
//
//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

const (
	defaultSSOBase    = "https://www.nazhisoft.com"
	defaultBizBase    = "http://139.159.205.146:8280"
	defaultUploadBase = "http://doc.nazhisoft.com"
	loginTimeout      = 90 * time.Second // OCR + 网络 + 99 次重试
	apiTimeout        = 30 * time.Second
)

// loadCreds 读取环境变量，未设置时调用 t.Skip 跳过。
func loadCreds(t *testing.T) (string, string, string, string) {
	t.Helper()
	username := os.Getenv("NAZHI_USERNAME")
	password := os.Getenv("NAZHI_PASSWORD")
	if username == "" || password == "" {
		t.Skip("跳过集成测试：未设置 NAZHI_USERNAME / NAZHI_PASSWORD 环境变量")
	}
	ssoBase := os.Getenv("NAZHI_SSO_BASE")
	if ssoBase == "" {
		ssoBase = defaultSSOBase
	}
	bizBase := os.Getenv("NAZHI_BASE_URL")
	if bizBase == "" {
		bizBase = defaultBizBase
	}
	return username, password, ssoBase, bizBase
}

// resolveOCRKey 解析验证码识别密钥：环境变量优先，回落 Nazhi-auto 本地配置。
// 与 e2e harness 同源逻辑（_test 符号无法跨包复用，测试代码允许这份轻度重复）。
func resolveOCRKey() string {
	for _, k := range []string{"NAZHI_SILICONFLOW_API_KEY", "NAZHI_OCR_API_KEY", "SILICONFLOW_API_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	for _, p := range []string{
		"E:/newCC/life-new2026/Nazhi-auto/backend/data/settings.yaml",
		"E:/newCC/life-new2026/Nazhi-auto/data/settings.yaml",
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		idx := strings.Index(string(data), "sk-")
		if idx < 0 {
			continue
		}
		tail := data[idx:]
		end := len(tail)
		for i, r := range tail {
			if r == '"' || r == '\'' || r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				if i > 3 {
					end = i
					break
				}
			}
		}
		return strings.Trim(string(tail[:end]), "\"' ")
	}
	return ""
}

// miniOmniOCR 是硅基流动 Qwen3-Omni 验证码识别的最小实现，
// 复刻 cmd/nazhi/omni_ocr.go 与 e2e harness 的同源逻辑（_test 符号无法跨包复用）。
type miniOmniOCR struct {
	apiKey string
	http   *http.Client
}

func (o *miniOmniOCR) Recognize(img []byte) (string, error) {
	if o == nil || o.apiKey == "" {
		return "", fmt.Errorf("Omni OCR 未配置 API Key")
	}
	if len(img) == 0 {
		return "", nil
	}
	body := map[string]any{
		"model": "Qwen/Qwen3-Omni-30B-A3B-Instruct",
		"messages": []any{
			map[string]any{"role": "system", "content": "Output 4 alphanumeric characters."},
			map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(img)}},
			}},
		},
		"temperature": 0,
		"max_tokens":  256,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequest(http.MethodPost, "https://api.siliconflow.cn/v1/chat/completions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// 用 map 解码避免匿名 struct 的 tag 书写；choices[0].message.content 可能是 string 或分段数组
	var parsed struct {
		Choices []map[string]any `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("Omni OCR 无返回")
	}
	msg, _ := parsed.Choices[0]["message"].(map[string]any)
	if msg == nil {
		return "", fmt.Errorf("Omni OCR 响应缺 message")
	}
	// content 可能是 string 或分段数组，统一 fmt 后只保留字母数字
	content := strings.TrimSpace(fmt.Sprintf("%v", msg["content"]))
	return strings.Join(strings.FieldsFunc(content, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
	}), ""), nil
}

// Close 实现 client.CaptchaRecognizer 接口（HTTP 客户端无资源需释放）。
func (o *miniOmniOCR) Close() error { return nil }

// newClient 构造一个真实环境 Client。
// 检测到 OCR 密钥时自动注入视觉识别器（环境变量或 Nazhi-auto 配置）。
func newClient(t *testing.T, ssoBase, bizBase string) *client.Client {
	t.Helper()
	opts := []client.Option{
		client.WithSSOBase(ssoBase),
		client.WithBaseURL(bizBase),
		client.WithUploadURL(defaultUploadBase),
		client.WithTimeout(apiTimeout),
	}
	if k := resolveOCRKey(); k != "" {
		opts = append(opts, client.WithCustomOCR(&miniOmniOCR{apiKey: k, http: &http.Client{Timeout: 45 * time.Second}}))
	}
	c, _ := client.New(opts...)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// sharedLogin 登录一次，所有测试复用 token。失败时 t.Fatal。
func sharedLogin(t *testing.T, c *client.Client, username, password string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	t.Logf("① 全自动 OCR 登录 (学号=%s)", maskUsername(username))
	resp, err := c.Login(ctx, types.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("登录成功但 token 为空")
	}
	t.Logf("✅ 登录成功，token 前缀: %s...", safePrefix(resp.Token, 20))
	return resp.Token
}

// maskUsername 部分遮罩学号用于日志。
func maskUsername(u string) string {
	if len(u) <= 4 {
		return strings.Repeat("*", len(u))
	}
	return u[:2] + strings.Repeat("*", len(u)-4) + u[len(u)-2:]
}

// safePrefix 安全地取字符串前缀（不 panic）。
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// truncate 截断字符串到指定长度。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// fmtMap 把 map 简单转字符串（用于日志）。
func fmtMap(m *map[string]any) string {
	if m == nil {
		return "<nil>"
	}
	parts := make([]string, 0, len(*m))
	for k, v := range *m {
		parts = append(parts, k+"="+truncate(anyToString(v), 20))
	}
	return strings.Join(parts, ", ")
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "<非字符串>"
}

// ────────────────────────────────────────────────────────────
// HAR 驱动的 Mock Server 测试（无需期末数据）
// ────────────────────────────────────────────────────────────

// harFixture 是从 HAR 文件提取的单条请求/响应。
type harFixture struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	RequestBody    string `json:"request_body"`
	ResponseStatus int    `json:"response_status"`
	ResponseBody   string `json:"response_body"`
}

// loadFixtures 加载 task_flow.json HAR fixtures。
func loadFixtures(t *testing.T) map[string]harFixture {
	t.Helper()
	return loadFixturesByName(t, "task_flow.json")
}

// loadFixturesByName 按文件名加载 fixtures（支持多种业务场景）。
func loadFixturesByName(t *testing.T, filename string) map[string]harFixture {
	t.Helper()
	path := filepath.Join("har_fixtures", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("加载 HAR fixtures 失败: %v（请确认 %s 存在，可从 Nazhi-auto/_archive 复制）", err, path)
	}
	var fixtures map[string]harFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("解析 fixtures 失败: %v", err)
	}
	return fixtures
}

// harMockServer 启动一个 mock HTTP server，按 HAR fixtures 返回真实响应。
// 同时记录实际请求，便于测试断言。
type harMockServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Path string
	Body string
}

// startHARMockServer 启动 mock server，按 HAR fixtures 返回真实响应。
// 同时为 ActivateSession 必需的端点添加 stub（仅当 HAR 未提供时）。
func startHARMockServer(t *testing.T, fixtures map[string]harFixture) *harMockServer {
	t.Helper()
	srv := &harMockServer{}
	mux := http.NewServeMux()
	srv.Server = httptest.NewServer(mux)

	// 注册所有 fixture 路径
	for _, fx := range fixtures {
		fx := fx // capture
		fullPath := fx.Path
		mux.HandleFunc(fullPath, func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			srv.mu.Lock()
			srv.requests = append(srv.requests, recordedRequest{
				Path: r.URL.Path,
				Body: string(body),
			})
			srv.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(fx.ResponseStatus)
			_, _ = w.Write([]byte(fx.ResponseBody))
			t.Logf("   mock ← %s %s → %d (%d bytes)", r.Method, r.URL.Path, fx.ResponseStatus, len(fx.ResponseBody))
		})
	}

	// ActivateSession 必需的 stub（仅当 HAR 未提供时）
	// 按 fixture 的 Path 字段判断，兼容 GET_api_x 与 GET__api_x 两种键名风格
	hasFixture := func(path string) bool {
		for _, fx := range fixtures {
			if fx.Path == path {
				return true
			}
		}
		return false
	}
	activateStubs := map[string]string{
		"/":                        `<html><body>home</body></html>`,
		"/api/studentInfo/getMenu": `{"code":1,"msg":"ok"}`,
		"/api/studentCircleNew/getCircleTypeByTaskId": `{"code":1,"msg":"ok","dataMap":{"task_id":16513,"circle_type_id":10,"dimension_id":3,"hours":32,"task_name":"stub","type_name":"stub","dimension_name":"stub","remark":"","type":1}}`,
		"/api/studentInfo/getMyInfo":                  `{"code":1,"msg":"成功","returnData":{"name":"测试用户","studentNumber":"TEST001","schoolName":"测试学校","className":"测试班级","seat":1}}`,
	}
	for path, body := range activateStubs {
		path := path
		body := body
		if hasFixture(path) {
			continue
		}
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(body))
			t.Logf("   stub ← %s %s → 200", r.Method, r.URL.Path)
		})
	}

	t.Logf("📦 HAR mock server: %s", srv.URL)
	return srv
}

func (s *harMockServer) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *harMockServer) Requests() []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]recordedRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

// TestReal_FullChain 端到端跑完所有能测的 SDK 方法。
// 不依赖期末数据，专注验证 SDK 与真实服务器的对齐度。
func TestReal_FullChain(t *testing.T) {
	username, password, ssoBase, bizBase := loadCreds(t)
	c := newClient(t, ssoBase, bizBase)

	// 1. 登录拿 token
	token := sharedLogin(t, c, username, password)
	if token == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	// 2. InitSession（已由 Login 内部调用，这里显式测一下）
	t.Log("② InitSession (SSO Session)")
	if err := c.InitSession(ctx); err != nil {
		t.Errorf("InitSession: %v", err)
	}

	// 3. GetSchoolID
	t.Log("③ GetSchoolID")
	school, err := c.GetSchoolID(ctx, username)
	if err != nil {
		t.Errorf("GetSchoolID: %v", err)
	} else {
		t.Logf("   ✅ 学校: %s (ID=%s)", school.SchoolName, school.SchoolID)
	}

	// 4. ActivateSession（4 步 HAR 对齐）
	t.Log("④ ActivateSession (4 步 HAR 对齐)")
	_, err = c.ActivateSession(ctx, token)
	if err != nil {
		t.Errorf("ActivateSession: %v", err)
	}

	// 5. GetMyInfo / whoami
	t.Log("⑤ GetMyInfo (whoami)")
	info, err := c.GetMyInfo(ctx, token)
	if err != nil {
		t.Errorf("GetMyInfo: %v", err)
	} else if info != nil {
		t.Logf("   ✅ %s / %s / %s (学号 %s)", info.Name, info.SchoolName, info.ClassName, info.StudentNumber)
	}

	// 6. GetDimensions（不需要任务）
	t.Log("⑥ GetDimensions (维度列表)")
	dims, err := c.GetDimensions(ctx, token)
	if err != nil {
		t.Errorf("GetDimensions: %v", err)
	} else {
		t.Logf("   ✅ %d 个维度", len(dims))
		for i, d := range dims {
			if i >= 3 {
				break
			}
			t.Logf("     - 维度 %d: %s", d.ID, d.Name)
		}
	}

	// 7. FetchTasks（期末未到，预期空列表）
	t.Log("⑦ FetchTasks (任务列表，期末未到预期空)")
	tasks, err := c.FetchTasks(ctx, token)
	if err != nil {
		t.Errorf("FetchTasks: %v", err)
	} else {
		t.Logf("   ✅ 任务数: %d（期末未到属正常）", len(tasks))
	}

	// 8. QuerySelfEvaluation（自我评价 + 教师评语）
	t.Log("⑧ QuerySelfEvaluation (自我评价 + 教师评语)")
	status, err := c.QuerySelfEvaluation(ctx, token)
	if err != nil {
		t.Errorf("QuerySelfEvaluation: %v", err)
	} else if status != nil {
		t.Logf("   ✅ 教师评语: %s", truncate(status.TeacherComment, 60))
	}

	// 9. QuerySelfGradEvaluation
	t.Log("⑨ QuerySelfGradEvaluation (学期评价)")
	grad, err := c.QuerySelfGradEvaluation(ctx, token)
	if err != nil {
		t.Logf("   ⚠️  QuerySelfGradEvaluation: %v", err)
	} else if grad != nil {
		t.Logf("   ✅ 学期评价: %v", truncate(fmtMap(grad), 80))
	} else {
		t.Log("   ℹ️  学期评价为空（正常）")
	}

	// 10. UploadFile（图片上传，5MB 压缩 + JPG 转换）
	t.Log("⑩ UploadFile (图片上传)")
	tmpImg := createTestImage(t)
	defer os.Remove(tmpImg)

	result, err := c.UploadFile(ctx, tmpImg)
	if err != nil {
		t.Errorf("UploadFile: %v", err)
	} else {
		t.Logf("   ✅ 上传成功，图片 ID: %d", result.AttachmentID)
	}
}

// ────────────────────────────────────────────────────────────
// TestHAR_FetchTasks 用真实 HAR 响应测试 FetchTasks
// （无需期末数据，因为响应是历史抓包）
// ────────────────────────────────────────────────────────────

// TestHAR_FetchTasks 用真 SDK + HAR 抓取的真实响应验证 FetchTasks 全链路。
//
// 数据来源：Nazhi-auto/_archive/综合评价破解/获取任务列表提交一次任务.har
// 该 HAR 抓取了真实期末期间的任务列表和提交请求。
func TestHAR_FetchTasks(t *testing.T) {
	fixtures := loadFixtures(t)
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	// 用真 SDK 跑！baseURL 指向 mock server
	c2, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token-for-har-test"),
	)
	t.Cleanup(func() { _ = c2.Close() })

	// 跑真 SDK 的 FetchTasks
	tasks, err := c2.FetchTasks(context.Background(), "fake-jwt-token-for-har-test")
	if err != nil {
		t.Fatalf("SDK FetchTasks 失败: %v", err)
	}

	if len(tasks) == 0 {
		t.Fatal("SDK FetchTasks 返回 0 任务，期望从 HAR 数据中解析出真实任务")
	}

	t.Logf("✅ SDK FetchTasks 解析出 %d 个任务（来自 HAR 真实响应）", len(tasks))
	for i, task := range tasks {
		if i >= 5 {
			t.Logf("   ... 还有 %d 个任务", len(tasks)-i)
			break
		}
		t.Logf("   - [%d] %s (dim=%s, hours=%.1f, submitted=%t)", task.ID, task.Name, task.DimensionName, task.Hours, task.Submitted)
	}

	// 验证 SDK 触发了正确的端点
	requests := srv.Requests()
	paths := make(map[string]int)
	for _, r := range requests {
		paths[r.Path]++
	}
	if paths["/api/studentCircleNew/getDimensions"] == 0 {
		t.Error("未触发 getDimensions")
	}
	if paths["/api/studentCircleNew/getCircleStatistics"] == 0 {
		t.Error("未触发 getCircleStatistics")
	}
	t.Logf("📊 SDK 触发了 %d 个端点:", len(paths))
	for p, n := range paths {
		t.Logf("   %s × %d", p, n)
	}
}

// TestHAR_FetchTasks_Debug 调试模式：直接验证 SDK 能否解析 HAR 响应
func TestHAR_FetchTasks_Debug(t *testing.T) {
	fixtures := loadFixtures(t)

	// 找一个 getCircleStatistics 响应
	var harBody []byte
	for k, v := range fixtures {
		if strings.Contains(k, "getCircleStatistics") {
			harBody = []byte(v.ResponseBody)
			break
		}
	}
	if harBody == nil {
		t.Fatal("HAR 中没有 getCircleStatistics 响应")
	}

	// 用 SDK 的 decoder 直接解析
	// 通过 http 客户端调 mock，拿到真实响应字节
	httpClient := &http.Client{Timeout: 5 * time.Second}
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	resp, err := httpClient.Get(srv.URL + "/api/studentCircleNew/getCircleStatistics?dimensionId=9")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	t.Logf("📦 响应长度: %d bytes (HAR 原长: %d bytes)", len(body), len(harBody))

	// 用 SDK 的统一响应解析
	parsed, err := types.DecodeResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	t.Logf("🔍 parsed.Code = %d", parsed.Code)
	t.Logf("🔍 parsed.DataList 是否为 nil: %v", parsed.DataList == nil)

	// 解析 task 列表
	tasks, err := types.DecodeDataList[types.Task](parsed)
	if err != nil {
		t.Fatalf("DecodeDataList: %v", err)
	}
	t.Logf("📊 SDK DecodeDataList 解析出 %d 个任务", len(tasks))
	if len(tasks) > 0 {
		first := tasks[0]
		t.Logf("   第一条: ID=%d, Name=%s, Submitted=%t", first.ID, first.Name, first.Submitted)
	}
}

// TestHAR_SubmitTask 用真 SDK + HAR 抓取的真实 addCircle 响应验证 SubmitTask。
func TestHAR_SubmitTask(t *testing.T) {
	fixtures := loadFixtures(t)
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token"),
	)
	t.Cleanup(func() { _ = c.Close() })

	input := types.TaskSubmitInput{
		TaskID:     16493,
		Content:    "通过参加爱党等相关活动...（真实内容约 100 字）",
		ImagePaths: []string{createTestImage(t)},
		Address:    "学校操场",
		Level:      "5",
		PlayRole:   "3",
	}

	t.Log("→ POST /api/studentCircleNew/addCircle (SDK SubmitTask)")
	result, err := c.SubmitTask(context.Background(), "fake-jwt-token", input)
	if err != nil {
		t.Fatalf("SDK SubmitTask 失败: %v", err)
	}

	if result.Code != 1 {
		t.Errorf("SubmitTask 返回 code=%d，期望 1", result.Code)
	}
	t.Logf("✅ SDK SubmitTask 成功，code=%d msg=%s", result.Code, result.Msg)

	// 验证 addCircle 被调用
	requests := srv.Requests()
	foundAddCircle := false
	for _, r := range requests {
		if r.Path == "/api/studentCircleNew/addCircle" {
			foundAddCircle = true
			t.Logf("📨 实际请求体: %s", truncate(r.Body, 200))
		}
	}
	if !foundAddCircle {
		t.Error("未触发 addCircle")
	}
}

// ────────────────────────────────────────────────────────────
// 4 个新场景测试：使用不同 HAR 的真实数据
// ────────────────────────────────────────────────────────────

// TestHAR_SubmitSelfEvaluation 用上传自我评价.har 验证 SubmitSelfEvaluation。
func TestHAR_SubmitSelfEvaluation(t *testing.T) {
	fixtures := loadFixturesByName(t, "self_eval.json")
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token"),
	)
	t.Cleanup(func() { _ = c.Close() })

	t.Log("→ POST /api/studentMoralEduNew/addSelfEvaluation (SDK SubmitSelfEvaluation)")
	err := c.SubmitSelfEvaluation(context.Background(), "fake-jwt-token", "这学期的表现很好，认真听讲，积极发言")
	if err != nil {
		t.Fatalf("SubmitSelfEvaluation: %v", err)
	}
	t.Logf("✅ SubmitSelfEvaluation 成功")

	// 验证 addSelfEvaluation 被调用
	requests := srv.Requests()
	foundSelf := false
	for _, r := range requests {
		if r.Path == "/api/studentMoralEduNew/addSelfEvaluation" {
			foundSelf = true
			t.Logf("📨 实际请求体: %s", r.Body)
		}
	}
	if !foundSelf {
		t.Error("未触发 addSelfEvaluation")
	}

	// 同时验证 querySelfEvaluation
	t.Log("→ GET /api/studentMoralEduNew/querySelfEvaluation (SDK QuerySelfEvaluation)")
	status, err := c.QuerySelfEvaluation(context.Background(), "fake-jwt-token")
	if err != nil {
		t.Errorf("QuerySelfEvaluation: %v", err)
	} else if status != nil {
		t.Logf("✅ QuerySelfEvaluation 返回: student_comment=%q", truncate(status.StudentComment, 30))
	}
}

// TestHAR_SubmitTask_Military 用军训.har 验证 SubmitTask 军训类型。
// 真实 payload: name="", level="", checkResult="1", hours=32, playRole=""
func TestHAR_SubmitTask_Military(t *testing.T) {
	fixtures := loadFixturesByName(t, "military.json")
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token"),
	)
	t.Cleanup(func() { _ = c.Close() })

	input := types.TaskSubmitInput{
		TaskID:     16513,
		Content:    "通过军训，我学会了坚持和团队合作...（真实心得约 100 字）",
		ImagePaths: []string{createTestImage(t)},
		Address:    "示例中学",
	}

	t.Log("→ POST /api/studentCircleNew/addCircle (军训类型)")
	result, err := c.SubmitTask(context.Background(), "fake-jwt-token", input)
	if err != nil {
		t.Fatalf("SubmitTask (军训): %v", err)
	}
	if result.Code != 1 {
		t.Errorf("返回 code=%d，期望 1", result.Code)
	}
	t.Logf("✅ 军训类型提交成功 code=%d", result.Code)
}

// TestHAR_SubmitTask_ClassMeeting 用班会.har 验证 SubmitTask 班会类型。
// 真实 payload: name="班会", level="", hours=1, playRole="3"
func TestHAR_SubmitTask_ClassMeeting(t *testing.T) {
	fixtures := loadFixturesByName(t, "class_meeting.json")
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token"),
	)
	t.Cleanup(func() { _ = c.Close() })

	input := types.TaskSubmitInput{
		TaskID:     16324,
		Content:    "今天班会我们讨论了诚信考试的重要性...（真实心得）",
		ImagePaths: []string{createTestImage(t)},
		Address:    "高一(8)班",
		PlayRole:   types.PlayRoleParticipant,
	}

	t.Log("→ POST /api/studentCircleNew/addCircle (班会类型)")
	result, err := c.SubmitTask(context.Background(), "fake-jwt-token", input)
	if err != nil {
		t.Fatalf("SubmitTask (班会): %v", err)
	}
	if result.Code != 1 {
		t.Errorf("返回 code=%d，期望 1", result.Code)
	}
	t.Logf("✅ 班会类型提交成功 code=%d", result.Code)
}

// TestHAR_SubmitTask_Labor 用劳动.har 验证 SubmitTask 劳动类型。
// 真实 payload: name="", level="5", hours=2, playRole=""
func TestHAR_SubmitTask_Labor(t *testing.T) {
	fixtures := loadFixturesByName(t, "labor.json")
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token"),
	)
	t.Cleanup(func() { _ = c.Close() })

	input := types.TaskSubmitInput{
		TaskID:     16512,
		Content:    "通过参加校园劳动，我体会到...（真实心得约 100 字）",
		ImagePaths: []string{createTestImage(t)},
		Address:    "示例中学",
		Level:      "5",
	}

	t.Log("→ POST /api/studentCircleNew/addCircle (劳动类型)")
	result, err := c.SubmitTask(context.Background(), "fake-jwt-token", input)
	if err != nil {
		t.Fatalf("SubmitTask (劳动): %v", err)
	}
	if result.Code != 1 {
		t.Errorf("返回 code=%d，期望 1", result.Code)
	}
	t.Logf("✅ 劳动类型提交成功 code=%d", result.Code)
}

// ────────────────────────────────────────────────────────────
// 辅助：创建测试图片
// ────────────────────────────────────────────────────────────

func createTestImage(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.png")

	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			alpha := uint8(255)
			if x > 400 && y > 300 {
				alpha = 128
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: uint8((x + y) % 256),
				A: alpha,
			})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建测试图片失败: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("PNG 编码失败: %v", err)
	}
	return path
}
