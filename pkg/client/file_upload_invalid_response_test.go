package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestUploadFile_HTMLResponse_InvalidResponse 锁定哨兵口径一致性：
// 上传服务器返回 200+HTML（WAF 挑战页/nginx 维护页）时必须归 ErrInvalidResponse，
// 与主管线 doBizAndDecode 的 200+非 JSON 口径拉平；此前裸 fmt.Errorf 让
// errors.Is 判定落空、CLI 漏斗走 default 500/exit2，与业务拒绝不可区分。
func TestUploadFile_HTMLResponse_InvalidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>WAF challenge</body></html>"))
	}))
	defer srv.Close()

	c, err := New(WithUploadURL(srv.URL))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}

	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = c.UploadFile(context.Background(), path)
	if err == nil {
		t.Fatal("200+HTML 应返回错误，实际 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("上传 200+非 JSON 应归 ErrInvalidResponse，实际: %v", err)
	}
}
