package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetMyInfo_ClassNameStripGrade 锁定 19 轮审计 user-info P2-2：
// postProcessUserInfo 对 className 只移除首个「级」字（user.go:102-104，
// 对齐前端 userBox.vue:70 / modifyBox.vue:175 / header.vue:260 的 replace("级","")），
// 此前全仓夹具均不含「级」字（client_test.go:430 "八班" / :467 "高一(8)班"），
// 该后处理零回归覆盖。含多个「级」字时仅删首个（JS replace 语义 1:1）。
func TestGetMyInfo_ClassNameStripGrade(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","className":"八年级(2)班"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer biz.Close()

	c := &Client{
		ssoBaseURL: biz.URL,
		baseURL:    biz.URL,
		uploadURL:  biz.URL,
		http:       newHTTPClient(),
		logger:     nil,
		ocr:        nil,
		sm:         &sessionManager{},
	}

	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if got := info.ClassName; got != "八年(2)班" {
		t.Errorf("className 应移除首个「级」字得到「八年(2)班」，实际 %q", got)
	}
}

// TestGetMyInfo_ClassNameNoGrade_KeepsOriginal 锁定无需去除时的行为：
// className 不含「级」字时保持原值（历史夹具形态不变）。
func TestGetMyInfo_ClassNameNoGrade_KeepsOriginal(t *testing.T) {
	biz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/", "/api/studentInfo/getMenu":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功"}`))
		case "/api/studentInfo/getMyInfo":
			_, _ = w.Write([]byte(`{"code":1,"msg":"成功","returnData":{"name":"张三","studentNumber":"TEST2025001","className":"高一(8)班"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer biz.Close()

	c := &Client{
		ssoBaseURL: biz.URL,
		baseURL:    biz.URL,
		uploadURL:  biz.URL,
		http:       newHTTPClient(),
		logger:     nil,
		ocr:        nil,
		sm:         &sessionManager{},
	}

	info, err := c.GetMyInfo(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("GetMyInfo 失败: %v", err)
	}
	if got := info.ClassName; got != "高一(8)班" {
		t.Errorf("className 不含「级」时应保持原值，实际 %q", got)
	}
}