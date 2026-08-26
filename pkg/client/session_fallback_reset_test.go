package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestActivateSession_TokenSwitch_RerunsSchoolFallback 锁定行为契约：
// fallbackDone 标志必须随「激活成功换新缓存」与「缓存失效」一起重置。
// 缺陷态（未修复）：token-A 激活完成学校回退后 fallbackDone=true；切换 token-B
// 激活成功走 RecordSuccess 但标志残留 true → 出口门控直接返回未经回退的 info，
// B 的 SchoolID/SchoolName 静默为空，直到一次同 token 激活失败才自愈。
func TestActivateSession_TokenSwitch_RerunsSchoolFallback(t *testing.T) {
	var ssoHits int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		atomic.AddInt32(&ssoHits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"7","NAME":"回退小学"}]}`))
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
			// 学号有值但缺 schoolId/schoolName → 每个新 token 都依赖出口门控的回退
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer biz.Close()

	c, err := New(
		WithBaseURL(biz.URL),
		WithSSOBase(sso.URL),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	// token-A：首次激活，应触发一次学校回退
	infoA, err := c.ActivateSession(context.Background(), "tok-A")
	if err != nil {
		t.Fatalf("tok-A 激活失败: %v", err)
	}
	if infoA.SchoolID == 0 || infoA.SchoolName == "" {
		t.Fatalf("tok-A 回退未生效: SchoolID=%d SchoolName=%q", infoA.SchoolID, infoA.SchoolName)
	}

	// token-B：换 token 再激活。RecordSuccess 写入全新缓存后，
	// 新缓存尚未经过学校回退，出口门控不得因残留标志跳过它。
	infoB, err := c.ActivateSession(context.Background(), "tok-B")
	if err != nil {
		t.Fatalf("tok-B 激活失败: %v", err)
	}
	if infoB.SchoolID == 0 || infoB.SchoolName == "" {
		t.Fatalf("tok-B 换 token 后学校回退被跳过（fallbackDone 未随 RecordSuccess 重置）: SchoolID=%d SchoolName=%q", infoB.SchoolID, infoB.SchoolName)
	}

	// 同 token fast path 重入：不应再次触发回退（幂等）
	if _, err := c.ActivateSession(context.Background(), "tok-B"); err != nil {
		t.Fatalf("tok-B 重入失败: %v", err)
	}
	if got := atomic.LoadInt32(&ssoHits); got != 2 {
		t.Errorf("SSO 回退应恰好执行 2 次（每个 token 各一次），实际 %d", got)
	}
}

// TestInvalidateCachedUserInfo_ResetsFallbackFlag 锁定：UpdateMyInfo 后缓存失效重建，
// 新缓存同样必须重新经过学校回退，不得因残留标志跳过。
func TestInvalidateCachedUserInfo_ResetsFallbackFlag(t *testing.T) {
	var ssoHits int32
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			atomic.AddInt32(&ssoHits, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"7","NAME":"回退小学"}]}`))
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
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		}
	}))
	defer biz.Close()

	c, err := New(
		WithBaseURL(biz.URL),
		WithSSOBase(sso.URL),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	if _, err := c.ActivateSession(context.Background(), "tok-A"); err != nil {
		t.Fatalf("tok-A 激活失败: %v", err)
	}

	c.sm.InvalidateCachedUserInfo()

	if _, err := c.ActivateSession(context.Background(), "tok-A"); err != nil {
		t.Fatalf("失效后重新激活失败: %v", err)
	}
	if got := atomic.LoadInt32(&ssoHits); got < 2 {
		t.Errorf("缓存失效重建后应再次执行学校回退（SSO 调用 >=2 次），实际 %d", got)
	}
}
