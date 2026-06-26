// Package client 内部白盒测试。
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetSchoolID_URLEncodesUsername 验证 GetSchoolID 对学号中的特殊字符进行 URL 编码。
//
// 历史 bug：auth.go:36 直接拼接 "?userName=" + username，若学号含 & / = 等
// 保留字符会破坏 URL 结构。此处传 "S123&456" 测试 & 被编码为 %26。
//
// 修复后：用 url.Values{"userName": {username}}.Encode() 构建 query，
// 与 session.go:107 模式对齐。
func TestGetSchoolID_URLEncodesUsername(t *testing.T) {
	var requestURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/teacher/auth/studentLogin/getSchoolIdByStudentNumber" {
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"100","NAME":"测试学校"}]}`))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c, _ := New(
		WithSSOBase(srv.URL),
		WithTimeout(5*time.Second),
	)

	_, _, err := c.GetSchoolID(context.Background(), "S123&456")
	if err != nil {
		t.Fatalf("GetSchoolID 失败: %v", err)
	}

	// 验证 userName 参数被正确编码（& 应变为 %26）
	if requestURL == "" {
		t.Fatal("请求未被发送")
	}
	if strings.Contains(requestURL, "userName=S123&456") {
		t.Errorf("userName 中的 & 未被编码: %s", requestURL)
	}
	if !strings.Contains(requestURL, "userName=S123%26456") {
		t.Errorf("期望 URL 编码 userName=S123%%26456，实际 URL: %s", requestURL)
	}
}
