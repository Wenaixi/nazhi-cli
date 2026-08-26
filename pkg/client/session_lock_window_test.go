package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestActivateSession_SchoolFallbackOutsideLock 锁定 P1-B 行为契约：
// 激活路径的学校信息 SSO 回退（getMyInfo 缺 schoolId/schoolName 时经 GetSchoolID 补全）
// 必须发生在 sm.mu 临界区之外。sm.Activate 的 godoc 承诺锁窗口约 200-500ms，
// 若在持锁状态下同步发起这次真实 SSO 域网络往返，最坏会被 c.http.Timeout 放大到
// 秒级，多 goroutine 并发调 ActivateSession 的公开模式全部被阻塞。
//
// 可观测断言：SSO handler 被调用时尝试 sm.mu.TryLock——
//   - 回退发生在锁内（缺陷态）→ TryLock 失败 → 本测试失败；
//   - 回退发生在锁外（修复态）→ TryLock 成功 → 通过。
//
// SSO 请求只会由 GetMyInfo 调用链发出，主 goroutine 此时必然已在 GetMyInfo 内，
// 不存在 c 尚未赋值的窗口。
func TestActivateSession_SchoolFallbackOutsideLock(t *testing.T) {
	var c *Client

	// SSO 域：GetSchoolID 回退端点。handler 内探测业务域 client 的 sm.mu 持有状态。
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			t.Errorf("SSO 域未预期路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if c == nil {
			t.Fatal("SSO handler 先于 client 构造被调用，夹具时序错误")
			return
		}
		// 关键断言：此刻 sm.mu 必须未被激活流程持有。
		if !c.sm.mu.TryLock() {
			t.Error("GetSchoolID 回退发生在 sm.mu 持锁期间——锁窗口被同步 SSO 往返放大")
		} else {
			c.sm.mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"1","NAME":"测试小学"}]}`))
	}))
	defer sso.Close()

	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"returnData":null}`))
		case "/api/studentInfo/getMyInfo":
			// 学号有值但缺 schoolId/schoolName → 触发 postProcessUserInfo 学校回退分支
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		default:
			t.Errorf("业务域未预期路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer biz.Close()

	client, err := New(
		WithBaseURL(biz.URL),
		WithSSOBase(sso.URL),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer client.Close()
	c = client

	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if info == nil || info.SchoolID == 0 || info.SchoolName == "" {
		t.Fatalf("学校回退应补全 SchoolID/SchoolName，实际: %+v", info)
	}
}
