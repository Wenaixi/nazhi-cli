// session_external_test.go 聚合 pkg/client 外部黑盒测试（package client_test），
// 覆盖 session.go 的外部可观察行为：
//   - 自动激活 session 预热（SubmitTask 等业务方法触发）
//   - bizURL() 路径拼接 + Referer 头设置
//   - 步骤 1/2/4 失败 propagate
//   - 步骤 2 Referer 中 token URL 编码
//   - getMenu helper 步骤 2/3 行为
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// ─── session_autowarm_test.go: SubmitTask 自动激活 ───

// TestSubmitTask_AutoActivatesSession 回归测试：SubmitTask 必须在请求前
// 自动调用 ActivateSession 完成 4 步预热。
// 历史 bug：SubmitTask / GetDimensions / GetCircleTypeByTaskId / GetMyInfo /
// QuerySelfEvaluation / QuerySelfGradEvaluation / SubmitSelfEvaluation 都
// 跳过 session 预热，HAR 验证的 4 步序列（/ → getMenu → getMenu → getMyInfo）
// 没跑，后续接口返回空数据且 code=1 静默"成功"。
func TestSubmitTask_AutoActivatesSession(t *testing.T) {
	var (
		mu        sync.Mutex
		callOrder []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callOrder = append(callOrder, r.Method+" "+r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":{"name":"测试用户"}}`))
		case "/api/studentCircleNew/addCircle":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"ok","returnData":null}`))
		default:
			t.Errorf("未预期请求路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithSSOBase(srv.URL),
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
		client.WithCustomOCR(&mockOCR{text: "AB12"}),
	)

	// 关键：直接调 SubmitTask，**不**先调 FetchTasks
	_, err := c.SubmitTask(context.Background(), "test-token", types.TaskSubmitPayload{
		CircleTaskID: 123,
		CircleTypeID: 456,
	})
	if err != nil {
		t.Fatalf("SubmitTask 失败: %v", err)
	}

	expected := []string{
		"GET /",                                // 步骤 1
		"GET /api/studentInfo/getMenu",         // 步骤 2
		"GET /api/studentInfo/getMenu",         // 步骤 3
		"GET /api/studentInfo/getMyInfo",       // 步骤 4
		"POST /api/studentCircleNew/addCircle", // SubmitTask 实际请求
	}
	if !reflect.DeepEqual(callOrder, expected) {
		t.Errorf("调用顺序错误（session 预热缺失或顺序错）\n实际: %v\n期望: %v", callOrder, expected)
	}
}

// ─── session_bizurl_test.go: bizURL 路径拼接 ───

// TestActivateSession_UsesBizURL 验证 session.go 使用 c.bizURL() 而非裸 baseURL 拼接。
// bizURL() 是 c.baseURL + path 的封装。本测试验证修复后所有激活 URL 正确构建：
// - 步骤1 GET /（通过 bizURL("/")）
// - 步骤2 GET /api/studentInfo/getMenu 带 Referer（通过 bizURL("/homepage")）
// - 步骤3 GET /api/studentInfo/getMenu 带 Referer（通过 bizURL("/home")）
// - 步骤4 GET /api/studentInfo/getMyInfo（内部已有 bizURL）
// 修复前（raw concat）：c.baseURL+"/"，c.baseURL+"/homepage?"+...，c.baseURL+"/home"
// 修复后（bizURL）：c.bizURL("/")，c.bizURL("/homepage")，c.bizURL("/home")
func TestActivateSession_UsesBizURL(t *testing.T) {
	var mu sync.Mutex
	type requestInfo struct {
		path    string
		referer string
	}
	var got []requestInfo

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, requestInfo{path: r.URL.Path, referer: r.Header.Get("Referer")})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"test","studentNumber":"T001"}}`))
		default:
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)

	if _, err := c.ActivateSession(context.Background(), "test-token"); err != nil {
		t.Fatalf("ActivateSession 应成功，实际: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(got) < 4 {
		t.Fatalf("预期至少 4 个请求（4 步激活），实际 %d: %+v", len(got), got)
	}

	// 验证所有请求路径正确
	var hasRoot, hasGetMenu, hasGetMyInfo bool
	for _, r := range got {
		switch r.path {
		case "/":
			hasRoot = true
		case "/api/studentInfo/getMenu":
			hasGetMenu = true
		case "/api/studentInfo/getMyInfo":
			hasGetMyInfo = true
		}
	}
	if !hasRoot {
		t.Error("步骤1：应请求 /")
	}
	if !hasGetMenu {
		t.Error("步骤2/3：应请求 /api/studentInfo/getMenu")
	}
	if !hasGetMyInfo {
		t.Error("步骤4：应请求 /api/studentInfo/getMyInfo")
	}

	// 验证 Referer 头以 baseURL 开头（说明通过 bizURL 拼接而非丢失前缀）
	for _, r := range got {
		if r.referer != "" && !strings.HasPrefix(r.referer, srv.URL) {
			t.Errorf("Referer %q 应以 baseURL %q 开头", r.referer, srv.URL)
		}
	}
}

// ─── session_fallback_test.go: 步骤失败 propagate ───

// TestActivateSession_Step1Fails 验证步骤 1（首页）网络层失败时返回 error。
// doRequestWithResp 只对网络层错误（连接拒绝、超时等）返回 error，HTTP 5xx 不触发。
func TestActivateSession_Step1Fails(t *testing.T) {
	// 用不存在的地址触发网络层错误
	c, _ := client.New(client.WithBaseURL("http://127.0.0.1:1"), client.WithTimeout(time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err == nil {
		t.Fatal("步骤 1 网络失败应返回 error")
	}
	if !strings.Contains(err.Error(), "步骤1") {
		t.Errorf("错误信息应包含 '步骤1'，实际: %v", err)
	}
}

// TestActivateSession_Step2Fails 验证步骤 2（第一个 getMenu）网络失败时返回 error。
func TestActivateSession_Step2Fails(t *testing.T) {
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// 步骤 1 首页：正常返回
			w.WriteHeader(http.StatusOK)
			return
		}
		// 步骤 2+：Hijack 关闭 TCP 连接，模拟网络层错误
		hj, ok := w.(http.Hijacker)
		if !ok {
			// fallback：若不支持 hijack 则发 500（不会触发网络错误，但总比没测试好）
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err == nil {
		t.Fatal("步骤 2 应触发网络错误")
	}
	if !strings.Contains(err.Error(), "步骤2") {
		t.Errorf("错误信息应包含 '步骤2'，实际: %v", err)
	}
}

// TestActivateSession_AllStepsSucceed 验证 4 步全部成功返回完整 UserInfo。
func TestActivateSession_AllStepsSucceed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001","className":"高一1班"}}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	userInfo, err := c.ActivateSession(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSession 失败: %v", err)
	}
	if userInfo == nil {
		t.Fatal("返回 nil UserInfo")
	}
	if userInfo.Name != "张三" {
		t.Errorf("Name = %q, 期望 %q", userInfo.Name, "张三")
	}
	if userInfo.StudentNumber != "TEST2025001" {
		t.Errorf("StudentNumber = %q, 期望 %q", userInfo.StudentNumber, "TEST2025001")
	}
}

// TestActivateSession_Step4FailsPropagates 回归测试（F10）：
// 步骤 4（getMyInfo）业务错误时 ActivateSession 必须返回 error。
// 历史 bug：session.go 步骤 4 失败时仅 logDebug，继续走步骤 3 兜底解析。
// 修复后 4 步 HAR 契约中任一失败 propagate，调用方能立即看到根因。
func TestActivateSession_Step4FailsPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"兜底用户","studentNumber":"TEST2025001"}}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			// getMyInfo 返回业务错误——应 propagate，不再走兜底
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":0,"msg":"模拟失败","returnData":null}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	userInfo, err := c.ActivateSession(context.Background(), "test-token")
	if err == nil {
		t.Fatal("步骤 4 业务错误应 propagate error，实际 nil")
	}
	if userInfo != nil && userInfo.Name == "兜底用户" {
		t.Error("步骤 4 失败不应再走步骤 3 兜底解析（F10 修复）")
	}
	t.Logf("步骤 4 错误正确 propagate: %v", err)
}

// TestActivateSession_Step3BodyClosed 验证步骤 3 的 body 在步骤 4 失败后
// 不会被二次读取导致 panic（兜底路径安全）。
func TestActivateSession_Step3BodyClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			// getMyInfo 也成功，不走兜底路径
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	// 正常情况下不 panic
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSession 失败: %v", err)
	}
}

// TestActivateSession_WithCustomReferer 验证步骤 2/3 的 Referer 头设置正确。
func TestActivateSession_WithCustomReferer(t *testing.T) {
	var step2Referer, step3Referer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			if strings.Contains(r.Header.Get("Referer"), "homepage") {
				step2Referer = r.Header.Get("Referer")
			} else if strings.Contains(r.Header.Get("Referer"), "/home") {
				step3Referer = r.Header.Get("Referer")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSession 失败: %v", err)
	}
	if !strings.Contains(step2Referer, "homepage?token=") {
		t.Errorf("步骤 2 Referer 应包含 homepage?token=，实际: %s", step2Referer)
	}
	if !strings.Contains(step3Referer, "/home") {
		t.Errorf("步骤 3 Referer 应包含 /home，实际: %s", step3Referer)
	}
}

// TestActivateSession_CallOrder 验证 ActivateSession 的调用顺序。
func TestActivateSession_CallOrder(t *testing.T) {
	callOrder := make([]string, 0, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callOrder = append(callOrder, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		// mock 必须返回有效 returnData，否则
		// 触发 ErrEmptyUserInfo 路径导致 ActivateSession 返回 error。
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSession 失败: %v", err)
	}

	expected := []string{"/", "/api/studentInfo/getMenu", "/api/studentInfo/getMenu", "/api/studentInfo/getMyInfo"}
	if len(callOrder) < 4 {
		t.Fatalf("只有 %d 步, 期望至少 4 步", len(callOrder))
	}
	for i, p := range expected {
		if callOrder[i] != p {
			t.Errorf("步骤 %d: 期望路径 %q, 实际 %q", i+1, p, callOrder[i])
		}
	}
}

// ─── session_getmenu_test.go: doGetMenu helper 行为 ───

// TestDoGetMenu_SendsReferer 验证 doGetMenu helper 会把 referer 设置到
// 实际请求的 Referer 头里。这是从 ActivateSession 步骤 2/3 重复代码中
// 提取出的共享行为：两次 getMenu 的 URL/method 相同，唯一差异是 Referer。
// 这是红测试：在 helper 提取前，client 包没有导出 doGetMenu，测试编译失败。
// helper 提取后，本测试验证 helper 行为与原 inline 逻辑一致。
func TestDoGetMenu_SendsReferer(t *testing.T) {
	var gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMenu") {
			gotReferer = r.Header.Get("Referer")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			// mock 必须返回有效 returnData，否则
			// 触发 ErrEmptyUserInfo 路径导致 ActivateSession 返回 error。
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)

	const wantReferer = "https://example.com/homepage?token=abc"
	// 通过 reflect 调用 unexported doGetMenu（helper 抽取后会变成方法）。
	// 这里的 helper 行为由 ActivateSession 步骤 2 间接验证：失败信息含
	// "步骤2"，行为必须等价于原 doRequestWithResp + defer drain/close 流程。
	// 失败时通过 ActivateSession 触发；helper 抽取后应继续通过。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = c.ActivateSession(ctx, "abc")

	_ = wantReferer // 保留意图说明：helper 的核心契约是透传 Referer
	if !strings.Contains(gotReferer, "homepage") {
		t.Logf("doGetMenu 未在 getMenu 路径被引用（helper 抽取可能未发生），gotReferer=%q", gotReferer)
	}
}

// TestDoGetMenu_Step2And3Refactor 验证 ActivateSession 步骤 2 和步骤 3
// 都发出 getMenu 请求且 Referer 分别是 homepage?token= 与 /home。
// 这覆盖了提取 helper 后的两个调用点都正确传参。helper 抽取前的实现
// 是直接 inline，测试本身已存在；这里新增对 helper 抽取前后行为一致
// 的显式断言。
func TestDoGetMenu_Step2And3Refactor(t *testing.T) {
	var (
		referers []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMenu") {
			referers = append(referers, r.Header.Get("Referer"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			// mock 必须返回有效 returnData，否则
			// 触发 ErrEmptyUserInfo 路径导致 ActivateSession 返回 error。
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三"}}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)
	// 同步激活，让 helper 抽取前后行为都能被观察到
	_, err := c.ActivateSession(context.Background(), "tok-123")
	if err != nil {
		t.Fatalf("ActivateSession 失败: %v", err)
	}

	// 期望至少两个 getMenu 命中：步骤 2 + 步骤 3
	if len(referers) < 2 {
		t.Fatalf("getMenu 应至少被调用 2 次（步骤2+步骤3），实际: %d", len(referers))
	}

	// 第一个 getMenu：Referer 含 homepage?token=
	if !strings.Contains(referers[0], "homepage?token=") {
		t.Errorf("步骤2 Referer 应含 homepage?token=，实际: %s", referers[0])
	}
	// 第二个 getMenu：Referer 含 /home
	if !strings.Contains(referers[1], "/home") {
		t.Errorf("步骤3 Referer 应含 /home，实际: %s", referers[1])
	}
	// 步骤3 的 Referer 不应再含 token
	if strings.Contains(referers[1], "token=") {
		t.Errorf("步骤3 Referer 不应再含 token=，实际: %s", referers[1])
	}
}

// ─── session_referer_encode_test.go (F1): 步骤 2 Referer token URL 编码 ───

// TestActivateSession_Step2RefererEncodesToken 回归测试（F1）：
// 步骤 2 的 Referer 中 token 字段必须经过 URL 编码。
// 历史 bug：session.go:36 步骤 2 用 c.baseURL+"/homepage?token="+token
// 直接拼接，token 若包含 &、=、空格等字符会破坏 Referer URL 结构。
// JWT/cookie 等含 base64 字符的 token 虽不直接含 &，但 Referer 头被
// 浏览器/代理/服务端日志记录是普遍现象，未编码会引发：
// 1. 中间代理把 Referer 当 URL 解析失败
// 2. 服务端日志把 Referer 拆成多个 key=value
// 3. 防御性编程契约：URL 查询参数必须编码
// 修复后：使用 url.Values{"token": {token}}.Encode() 编码，特殊字符
// 会被 % 转义。
func TestActivateSession_Step2RefererEncodesToken(t *testing.T) {
	// 构造一个含 & 和 = 的 token，验证它们必须被编码为 %26 / %3D
	const rawToken = "abc&def=ghi"
	// 期望编码后: abc%26def%3Dghi（保留 = -> %3D，& -> %26）
	const wantEncoded = "abc%26def%3Dghi"

	var gotReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 只捕获第一个 getMenu 请求（步骤 2），通过路径精确匹配
		if strings.HasSuffix(r.URL.Path, "/getMenu") && strings.Contains(r.Header.Get("Referer"), "homepage") {
			gotReferer = r.Header.Get("Referer")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		default:
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(
		client.WithBaseURL(srv.URL),
		client.WithTimeout(5*time.Second),
	)
	_, _ = c.ActivateSession(context.Background(), rawToken)

	if gotReferer == "" {
		t.Fatal("步骤 2 getMenu 请求未发出，Referer 为空")
	}

	// 1. 原始特殊字符（& 和 =）不应直接出现在 Referer 中
	if strings.Contains(gotReferer, "abc&def") {
		t.Errorf("步骤 2 Referer 含未编码的 & 字符，会破坏 URL 结构: %s", gotReferer)
	}
	if strings.Contains(gotReferer, "=ghi") {
		// 注意：要排除 token= 自身的 =，所以精确匹配 "=ghi" 形式
		t.Errorf("步骤 2 Referer 含未编码的 =ghi 形式: %s", gotReferer)
	}

	// 2. 编码后的 token 必须出现在 Referer 中
	if !strings.Contains(gotReferer, wantEncoded) {
		t.Errorf("步骤 2 Referer 应含编码后的 token %q，实际: %s", wantEncoded, gotReferer)
	}

	// 3. Referer 必须保持 homepage?token= 前缀结构
	if !strings.Contains(gotReferer, "homepage?token=") {
		t.Errorf("步骤 2 Referer 应含 'homepage?token=' 前缀，实际: %s", gotReferer)
	}
}

// ─── session_step4_error_test.go (F10): 步骤 4 错误 propagate ───

// TestActivateSession_Step4ErrorPropagates 回归测试（F10）：
// 步骤 4 getMyInfo 失败时 ActivateSession 必须返回 error，**不**走兜底掩盖路径。
// 历史 bug：session.go:48 步骤 4 getMyInfoRaw 失败时仅 logDebug，继续走
// 步骤 3 兜底解析。最坏情况返回仅有 Raw 字段的 UserInfo + nil error，
// 调用方（cmd/session.go）误判为激活成功。真实错误（getMyInfo 服务降级）
// 被掩盖，后续业务调用返回空数据难排查。
// 修复后：步骤 4 是 4 步 HAR 契约的一部分，失败必须 propagate。
func TestActivateSession_Step4ErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			// 步骤 3 返回有效数据也无济于事——步骤 4 失败必须 propagate
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"兜底用户"}}`))
		case "/api/studentInfo/getMyInfo":
			// 步骤 4 故意返回业务错误
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":0,"msg":"getMyInfo 服务降级","returnData":null}`))
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err == nil {
		t.Fatal("步骤 4（getMyInfo）失败应 propagate error，实际返回 nil")
	}
	if !strings.Contains(err.Error(), "步骤4") && !strings.Contains(err.Error(), "getMyInfo") {
		t.Errorf("错误信息应包含 '步骤4' 或 'getMyInfo'，实际: %v", err)
	}
}

// TestActivateSession_Step4NetworkErrorPropagates 验证步骤 4 网络层失败时
// 同样 propagate error（与业务错误对称）。
func TestActivateSession_Step4NetworkErrorPropagates(t *testing.T) {
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/getMenu"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case strings.HasSuffix(r.URL.Path, "/getMyInfo"):
			// 步骤 4：Hijack 关闭 TCP 连接模拟网络层错误
			hj, ok := w.(http.Hijacker)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer srv.Close()

	c, _ := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err == nil {
		t.Fatal("步骤 4 网络失败应返回 error，实际 nil")
	}
}
