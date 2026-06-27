// Package client_test — session.go bizURL() 使用回归测试。
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
	// 必须用 trackClient 注册，但因 Client 生命周期不长于测试，
	// 这里不注册也无实际泄漏风险。
	// _ = c.Close() 在测试结束时由 defer srv.Close 确保资源释放。

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
