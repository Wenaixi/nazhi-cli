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

// TestActivateSession_Step4ErrorPropagates 回归测试（F10）：
// 步骤 4 getMyInfo 失败时 ActivateSession 必须返回 error，**不**走兜底掩盖路径。
//
// 历史 bug：session.go:48 步骤 4 getMyInfoRaw 失败时仅 logDebug，继续走
// 步骤 3 兜底解析。最坏情况返回仅有 Raw 字段的 UserInfo + nil error，
// 调用方（cmd/session.go）误判为激活成功。真实错误（getMyInfo 服务降级）
// 被掩盖，后续业务调用返回空数据难排查。
//
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

	c := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(5*time.Second))
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

	c := client.New(client.WithBaseURL(srv.URL), client.WithTimeout(time.Second))
	_, err := c.ActivateSession(context.Background(), "test-token")
	if err == nil {
		t.Fatal("步骤 4 网络失败应返回 error，实际 nil")
	}
}
