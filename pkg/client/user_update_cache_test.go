package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

// TestUpdateMyInfo_InvalidatesCachedUserInfo 回归：UpdateMyInfo 成功后必须
// 清空 sm.cachedUserInfo，否则下次 GetMyInfo 会继续返回更新前的缓存。
//
// 历史 bug：ActivateSession 步骤 4 缓存 UserInfo；UpdateMyInfo 只写服务端，
// 不失效本地缓存，同进程后续 GetMyInfo 走 DCL fast path 读到旧数据。
func TestUpdateMyInfo_InvalidatesCachedUserInfo(t *testing.T) {
	var myInfoCount int32
	var updateCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			n := atomic.AddInt32(&myInfoCount, 1)
			w.WriteHeader(http.StatusOK)
			if n == 1 {
				_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"旧名字","studentNumber":"TEST2025001","telephone":"10000000000"}}`))
			} else {
				_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"新名字","studentNumber":"TEST2025001","telephone":"13800138000"}}`))
			}
		case "/api/studentInfo/updateMyInfo":
			if r.Method != http.MethodPost {
				t.Errorf("updateMyInfo 期望 POST, 得到 %s", r.Method)
			}
			atomic.AddInt32(&updateCount, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		default:
			t.Errorf("未预期路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	// 首次 GetMyInfo：激活 session，缓存 UserInfo
	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("首次 GetMyInfo 失败: %v", err)
	}
	if info == nil || info.Name != "旧名字" {
		t.Fatalf("首次 GetMyInfo 期望 name=旧名字, 得到 %+v", info)
	}
	if n := atomic.LoadInt32(&myInfoCount); n != 1 {
		t.Fatalf("首次 getMyInfo 请求次数期望 1, 实际 %d", n)
	}
	if c.sm.cachedUserInfo == nil {
		t.Fatal("激活后 sm.cachedUserInfo 应非 nil")
	}

	// 更新个人信息
	if err := c.UpdateMyInfo(context.Background(), "test-token", map[string]any{
		"telephone": "13800138000",
	}); err != nil {
		t.Fatalf("UpdateMyInfo 失败: %v", err)
	}
	if n := atomic.LoadInt32(&updateCount); n != 1 {
		t.Fatalf("updateMyInfo 请求次数期望 1, 实际 %d", n)
	}
	if c.sm.cachedUserInfo != nil {
		t.Fatal("UpdateMyInfo 成功后 sm.cachedUserInfo 应被清空")
	}

	// 再次 GetMyInfo：缓存已失效，必须重新拉
	info2, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("二次 GetMyInfo 失败: %v", err)
	}
	if info2 == nil || info2.Name != "新名字" {
		t.Fatalf("二次 GetMyInfo 期望 name=新名字（重新拉取）, 得到 %+v", info2)
	}
	if n := atomic.LoadInt32(&myInfoCount); n != 2 {
		t.Errorf("UpdateMyInfo 后二次 GetMyInfo 应再请求 getMyInfo，期望 2 次, 实际 %d", n)
	}
}

// TestInvalidateCachedUserInfo_Public 验证公开方法可主动清缓存。
func TestInvalidateCachedUserInfo_Public(t *testing.T) {
	c, err := New(WithTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	c.sm.mu.Lock()
	c.sm.cachedUserInfo = &types.UserInfo{Name: "cached"}
	c.sm.mu.Unlock()

	c.InvalidateCachedUserInfo()
	if c.sm.cachedUserInfo != nil {
		t.Fatal("InvalidateCachedUserInfo 后 cachedUserInfo 应为 nil")
	}
}
