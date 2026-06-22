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
	"context"
	"encoding/json"
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

// newClient 构造一个真实环境 Client。
func newClient(t *testing.T, ssoBase, bizBase string) *client.Client {
	t.Helper()
	c := client.New(
		client.WithSSOBase(ssoBase),
		client.WithBaseURL(bizBase),
		client.WithUploadURL(defaultUploadBase),
		client.WithTimeout(apiTimeout),
	)
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
	path := filepath.Join("har_fixtures", "task_flow.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("加载 HAR fixtures 失败: %v（请确认 %s 存在）", err, path)
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
	hasFixture := func(path string) bool {
		_, ok := fixtures["GET_"+strings.TrimPrefix(path, "/")]
		return ok
	}
	activateStubs := map[string]string{
		"/":                                    `<html><body>home</body></html>`,
		"/api/studentInfo/getMyInfo":           `{"code":1,"msg":"成功","returnData":{"name":"测试用户","studentNumber":"TEST001","schoolName":"测试学校","className":"测试班级","seat":1}}`,
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
	schoolID, schoolName, err := c.GetSchoolID(ctx, username)
	if err != nil {
		t.Errorf("GetSchoolID: %v", err)
	} else {
		t.Logf("   ✅ 学校: %s (ID=%s)", schoolName, schoolID)
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
		t.Logf("   ✅ %s / %s / %s / 座号 %d", info.Name, info.SchoolName, info.ClassName, info.Seat)
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

	id, err := c.UploadFile(ctx, tmpImg)
	if err != nil {
		t.Errorf("UploadFile: %v", err)
	} else {
		t.Logf("   ✅ 上传成功，图片 ID: %d", id)
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
	c2 := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token-for-har-test"),
	)

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
		t.Logf("   - [%d] %s (dim=%d, hours=%.1f, status=%s)", task.ID, task.Name, task.DimensionID, task.Hours, task.Status)
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
		t.Logf("   第一条: ID=%d, Name=%s, Status=%s", first.ID, first.Name, first.Status)
	}
}

// TestHAR_SubmitTask 用真 SDK + HAR 抓取的真实 addCircle 响应验证 SubmitTask。
func TestHAR_SubmitTask(t *testing.T) {
	fixtures := loadFixtures(t)
	srv := startHARMockServer(t, fixtures)
	defer srv.Close()

	c := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(10*time.Second),
		client.WithToken("fake-jwt-token"),
	)

	// 用真实 HAR addCircle 的字段（解码 GBK 中文）
	payload := types.TaskSubmitPayload{
		ID:                 nil,
		Name:               "示例任务名",
		HostName:           "",
		CircleDate:         "",
		Rank:               "",
		Level:              "",
		Content:            "通过参加爱党等相关活动...（真实内容约 100 字）",
		PictureList:        []int64{4384402},
		CircleTaskID:       16493,
		CircleTypeID:       9255,
		DimensionID:        9,
		Hours:              0.5,
		CircleBeginDate:    "",
		CircleEndDate:      "",
		CheckResult:        "",
		PatentType:         "",
		PatentNum:          "",
		Address:            "学校操场",
		TermName:           "",
		ActivityName:       "",
		SportsName:         "",
		TeamName:           "",
		OrgName:            "",
		ResultsName:        "",
		ObtainTime:         "",
		SpecialtyTechnology: "",
		PlayRole:           "3",
		LikeSpecialty1:     "",
		LikeSpecialty2:     "",
		LikeSpecialty3:     "",
	}

	t.Log("→ POST /api/studentCircleNew/addCircle (SDK SubmitTask)")
	result, err := c.SubmitTask(context.Background(), "fake-jwt-token", payload)
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
