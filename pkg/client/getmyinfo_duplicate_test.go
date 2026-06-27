// Package client 内部白盒测试。
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestGetMyInfo_NoDuplicateRequest 验证 GetMyInfo 在 session 激活后不重复
// 调用 getMyInfoRaw。激活步骤 4 已拉取数据，GetMyInfo 应复用该结果。
// 历史 bug：activateSessionIfNeeded 丢弃了步骤 4（getMyInfoRaw）返回的
// UserInfo，GetMyInfo 在 session 激活后再次调用 getMyInfoRaw，多一次
// HTTP 请求。
// 修复后：activateSessionIfNeeded 返回 UserInfo，GetMyInfo 在激活后
// 检查返回值，若非 nil 则直接返回，不额外请求。
func TestGetMyInfo_NoDuplicateRequest(t *testing.T) {
	var myInfoCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			atomic.AddInt32(&myInfoCount, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithBaseURL(srv.URL),
		WithTimeout(5*time.Second),
	)

	// 首次调用 GetMyInfo：应触发 4 步激活，步骤 4 已拿数据，不应额外请求
	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if info == nil {
		t.Fatal("GetMyInfo 返回 nil")
	}

	if n := atomic.LoadInt32(&myInfoCount); n != 1 {
		t.Errorf("getMyInfo HTTP 请求次数期望 1 次（步骤 4），实际 %d 次 — GetMyInfo 有重复请求", n)
	}

	// 再次调用 GetMyInfo（session 已激活，fast path）：不应有任何请求
	info2, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("二次 GetMyInfo 失败: %v", err)
	}
	if info2 == nil {
		t.Fatal("二次 GetMyInfo 返回 nil")
	}

	if n := atomic.LoadInt32(&myInfoCount); n != 1 {
		t.Errorf("session 已激活后应无额外 getMyInfo 请求，实际 %d 次", n)
	}
}
