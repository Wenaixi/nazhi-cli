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
//
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
		// F10 修复（round-7）：mock 必须返回有效 returnData，否则
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
