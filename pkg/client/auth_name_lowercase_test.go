package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetSchoolID_NameLowercaseKey 锁定 P2-3：GetSchoolID 学校名键兼容小写 name。
// 服务端 school_id 用小写键、NAME 用大写键，命名风格不一致；部分部署可能返回小写 name。
// SDK 双键读取（NAME 优先，name 兜底），保证两种形态都能拿到学校名。
func TestGetSchoolID_NameLowercaseKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/uiStudentLogin/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>ok</html>"))
		case "/teacher/auth/studentLogin/getSchoolIdByStudentNumber":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":1,"dataList":[{"school_id":"100","name":"小写键学校"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{
		ssoBaseURL: srv.URL,
		baseURL:    srv.URL,
		uploadURL:  srv.URL,
		http:       newHTTPClient(),
	}
	info, err := c.GetSchoolID(context.Background(), "TESTUSER20260825")
	if err != nil {
		t.Fatalf("GetSchoolID 失败: %v", err)
	}
	if info.SchoolName != "小写键学校" {
		t.Errorf("应从小写 name 键读取学校名，实际 SchoolName=%q", info.SchoolName)
	}
}
