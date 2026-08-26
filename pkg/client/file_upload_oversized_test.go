package client

import (
	"context"
	"errors"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUploadFile_OversizedSuccessBody_Rejects 锁定 19 轮审计 file-upload P2-1/P2-3：
// 上传成功路径响应体必须封顶 1MB（HTTP-2 契约对齐 request.go），超限归 ErrInvalidResponse
// 而非继续全量读入；且超限分支必须直 Close 放弃 keep-alive（对齐 httpDo:377-381 的
// 2356484 修复纪律），不再经 defer drainAndClose 无上限续读剩余 body。
func TestUploadFile_OversizedSuccessBody_Rejects(t *testing.T) {
	// 先构造一个合法 JPEG 文件（避免预处理阶段 ErrInvalidPayload）
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "tiny.jpg")
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 50}); err != nil {
		t.Fatalf("编码 JPEG 失败: %v", err)
	}
	_ = f.Close()

	huge := strings.Repeat("a", 2<<20) // 2MB 非 JSON 超大 body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, huge)
	}))
	defer srv.Close()

	c, err := New(WithUploadURL(srv.URL), WithSSOBase(srv.URL), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.UploadFile(context.Background(), imgPath)
	if err == nil {
		t.Fatal("超大响应体应报错，实际 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("超大响应体应归 ErrInvalidResponse，实际 %v", err)
	}
}