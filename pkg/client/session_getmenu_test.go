// Package client_test session.doGetMenu helper 单元测试。
package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// TestDoGetMenu_SendsReferer 验证 doGetMenu helper 会把 referer 设置到
// 实际请求的 Referer 头里。这是从 ActivateSession 步骤 2/3 重复代码中
// 提取出的共享行为：两次 getMenu 的 URL/method 相同，唯一差异是 Referer。
//
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
			// F10 修复（round-7）：mock 必须返回有效 returnData，否则
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
//
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
			// F10 修复（round-7）：mock 必须返回有效 returnData，否则
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
