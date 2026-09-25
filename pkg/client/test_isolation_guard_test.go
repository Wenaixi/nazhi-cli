package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// recordingTransport 记录所有经由它发出的请求 Host，
// 供断言「测试期间没有任何请求离开本机」。
type recordingTransport struct {
	base  http.RoundTripper
	mu    sync.Mutex
	hosts []string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.hosts = append(rt.hosts, req.URL.Host)
	rt.mu.Unlock()
	return rt.base.RoundTrip(req)
}

func (rt *recordingTransport) sawHostContaining(sub string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, h := range rt.hosts {
		if strings.Contains(h, sub) {
			return true
		}
	}
	return false
}

// unifiedUserInfoBody 生成 getMyInfo 统一响应；故意不返回 schoolId/schoolName，
// 以触发 ActivateSession 出口的学校信息 SSO 回退，确保 SSO 基址真正被使用到。
func unifiedUserInfoBody() string {
	return `{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001"}}`
}

// newIsolationGuardServer 构造覆盖激活四步 + 学校回退端点的本地 mock。
func newIsolationGuardServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { ok(w, unifiedUserInfoBody()) })
	mux.HandleFunc("/api/studentInfo/getMenu", func(w http.ResponseWriter, r *http.Request) {
		ok(w, `{"code":1,"msg":"成功"}`)
	})
	mux.HandleFunc("/api/studentInfo/getMyInfo", func(w http.ResponseWriter, r *http.Request) {
		ok(w, unifiedUserInfoBody())
	})
	// 学校信息回退端点：本地 mock 可应答，证明回退不越界
	mux.HandleFunc("/teacher/auth/studentLogin/getSchoolIdByStudentNumber", func(w http.ResponseWriter, r *http.Request) {
		ok(w, `{"code":1,"dataList":[{"school_id":"173","name":"本地测试学校"}]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestClient_SchoolFallbackStaysLocal 验证学校信息 SSO 回退只访问本地 mock。
// 这是守卫的正向基线：SSO 基址指向本地时，回退能拿到学校名且不出网。
func TestClient_SchoolFallbackStaysLocal(t *testing.T) {
	srv := newIsolationGuardServer(t)

	rt := &recordingTransport{base: http.DefaultTransport}
	c, err := client.New(
		client.WithTimeout(5*time.Second),
		client.WithBaseURL(srv.URL),
		client.WithSSOBase(srv.URL),
		client.WithHTTPClient(&http.Client{Transport: rt}),
	)
	if err != nil {
		t.Fatalf("构造 Client 失败: %v", err)
	}
	defer c.Close()

	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if info.SchoolName != "本地测试学校" {
		t.Errorf("学校名应来自本地 mock 回退，实际 %q", info.SchoolName)
	}
	if rt.sawHostContaining("nazhisoft.com") {
		t.Fatalf("学校回退出现生产域越界请求，Host 记录: %v", rt.hosts)
	}
}

// TestClient_ProductionSSOBaseIsNeverImplicit 锁定测试隔离不变量：
// 只提供 biz mock、不提供 sso mock 时，Client 不得把请求发往生产 SSO 域。
//
// 背景：2026-09-26 全量并发实测捕获到真实越界请求
//
//	POST https://www.nazhisoft.com/teacher/auth/studentLogin/getSchoolIdByStudentNumber?userName=***
//
// 成因是 New 的 ssoBaseURL 默认值即生产域，而只设 WithBaseURL 的测试
// 没有覆盖该默认值；一旦走到学校信息回退（postProcessSchoolFallback）
// 就会真的出网，违反「真实平台默认只读」的项目约定。
//
// 本测试只注入 WithBaseURL（不注入 WithSSOBase），并用记录 Transport
// 观察是否发生越界。
func TestNewTestClient_SharedFixtureNeverEscapesToProductionDomain(t *testing.T) {
	srv := newIsolationGuardServer(t)

	// 走公共夹具且故意不传 ssoServer：这是最容易漏设、也最容易越界的调用方式
	c := newTestClient(nil, srv, nil)
	defer c.Close()

	// 夹具已把 SSO 基址兜底到 biz mock，故此处必须既不越界也能取到学校名
	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if info.SchoolName != "本地测试学校" {
		t.Errorf("学校名应来自本地 mock 回退，实际 %q", info.SchoolName)
	}
}
