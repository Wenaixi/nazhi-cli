package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestActivateSessionJSON_SchoolFallback 锁定 AUTH-1 契约：
// ActivateSessionJSON 的 godoc 承诺"包含学校信息 SSO 降级补全"，
// 直接调 sm.Activate 后 Marshal 的实现不会触发 postProcessSchoolFallback，
// 输出 JSON 中 schoolId/schoolName 缺失——与 godoc 失实。
// 修复后应改调 GetMyInfo 语义（ActivateSession 出口已做学校回退）。
func TestActivateSessionJSON_SchoolFallback(t *testing.T) {
	sso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			t.Errorf("SSO 域未预期路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
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
			// 学号有值但缺 schoolId/schoolName → 应触发学校回退补全
			_, _ = w.Write([]byte(`{"code":1,"returnData":{"name":"张三","studentNumber":"TEST2025001"}}`))
		default:
			t.Errorf("业务域未预期路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
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

	raw, err := c.ActivateSessionJSON(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ActivateSessionJSON 失败: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("ActivateSessionJSON 返回空 JSON")
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("输出非合法 JSON: %v", err)
	}
	// AUTH-1 红线：学校信息必须被补全（godoc 承诺）
	if sid, _ := out["schoolId"].(float64); sid != 1 {
		t.Errorf("schoolId 未补全，期望 1 实际 %v", out["schoolId"])
	}
	if sn, _ := out["schoolName"].(string); sn != "测试小学" {
		t.Errorf("schoolName 未补全，期望 测试小学 实际 %q", out["schoolName"])
	}
}
