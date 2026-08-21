package client

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestUploadFile_MissingFile_NotErrFileTooLarge 验证路径不存在时不应 errors.Is(ErrFileTooLarge)。
// 回归：UploadFile 曾对 prepareImageForUpload 任意错误 Join ErrFileTooLarge，
// 导致调用方把「文件不存在/解码失败」误判为「文件过大」。
func TestUploadFile_MissingFile_NotErrFileTooLarge(t *testing.T) {
	c, _ := New(WithUploadURL("http://127.0.0.1:9"), WithTimeout(2*time.Second))
	_, err := c.UploadFile(t.Context(), filepath.Join(t.TempDir(), "does-not-exist.png"))
	if err == nil {
		t.Fatal("期望路径不存在时返回错误，实际 nil")
	}
	if errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("路径不存在不应 Is(ErrFileTooLarge)，实际: %v", err)
	}
}

// TestUploadFile_DecodeFail_NotErrFileTooLarge 验证非图片/解码失败不应 Is(ErrFileTooLarge)。
func TestUploadFile_DecodeFail_NotErrFileTooLarge(t *testing.T) {
	c, _ := New(WithUploadURL("http://127.0.0.1:9"), WithTimeout(2*time.Second))
	bad := filepath.Join(t.TempDir(), "not-image.bin")
	if err := os.WriteFile(bad, []byte("this is not an image"), 0o644); err != nil {
		t.Fatalf("写测试文件失败: %v", err)
	}
	_, err := c.UploadFile(t.Context(), bad)
	if err == nil {
		t.Fatal("期望解码失败返回错误，实际 nil")
	}
	if errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("解码失败不应 Is(ErrFileTooLarge)，实际: %v", err)
	}
}

// TestUploadFile_FormFilename_UsesBaseJpg 验证 multipart filename 仅用 basename 且扩展名为 .jpg，
// 不含本地绝对路径。预处理始终输出 JPEG，filename 应统一 .jpg。
func TestUploadFile_FormFilename_UsesBaseJpg(t *testing.T) {
	var gotBody []byte
	var gotCT string
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":42}}`))
	}))
	defer upload.Close()

	c, _ := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))

	// 嵌套目录 + 非 jpg 扩展名，制造「完整路径」场景
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tmpfile := filepath.Join(dir, "photo.png")
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{1, 2, 3, 255})
		}
	}
	f, err := os.Create(tmpfile)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode: %v", err)
	}
	f.Close()

	if _, err := c.UploadFile(t.Context(), tmpfile); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	_, params, err := mime.ParseMediaType(gotCT)
	if err != nil {
		t.Fatalf("ParseMediaType: %v ct=%q", err, gotCT)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatalf("缺少 boundary: %q", gotCT)
	}
	mr := multipart.NewReader(bytes.NewReader(gotBody), boundary)
	part, err := mr.NextPart()
	if err != nil {
		t.Fatalf("NextPart: %v", err)
	}
	filename := part.FileName()
	if filename == "" {
		t.Fatal("multipart FileName 为空")
	}
	if strings.Contains(filename, string(os.PathSeparator)) || strings.Contains(filename, "/") || strings.Contains(filename, `\`) {
		t.Errorf("filename 不应含路径分隔符，实际 %q", filename)
	}
	if filepath.Base(tmpfile) == filename {
		// 旧 bug 可能是 fullpath+".jpg"，也可能 basename 仍带 .png
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".jpg") {
		t.Errorf("filename 应统一 .jpg 扩展名，实际 %q", filename)
	}
	if filename != "photo.jpg" {
		t.Errorf("filename 期望 photo.jpg，实际 %q", filename)
	}
}

// TestMultipartBufPool_PreallocatesMaxImageSize 验证 pool New 预分配 ≥ MaxImageSize+1024。
// 回归：曾写成 5*1024+1024=6KB，注释却声称 5MB。
func TestMultipartBufPool_PreallocatesMaxImageSize(t *testing.T) {
	if multipartBufPool.New == nil {
		t.Fatal("multipartBufPool.New 未设置")
	}
	raw := multipartBufPool.New()
	buf, ok := raw.(*bytes.Buffer)
	if !ok || buf == nil {
		t.Fatalf("New 应返回 *bytes.Buffer，实际 %T", raw)
	}
	want := MaxImageSize + 1024
	if buf.Cap() < want {
		t.Errorf("multipartBufPool 预分配 Cap=%d，期望 >= %d（MaxImageSize+1024）", buf.Cap(), want)
	}
}

// TestHasHostSuffix_RejectsSuffixCollision 验证 evilnazhisoft.com 不能匹配 nazhisoft.com。
// 回归：host[len-len(suffix):]==suffix 允许无点号前缀的后缀碰撞。
func TestHasHostSuffix_RejectsSuffixCollision(t *testing.T) {
	cases := []struct {
		host, suffix string
		want         bool
	}{
		{"nazhisoft.com", "nazhisoft.com", true},
		{"doc.nazhisoft.com", "nazhisoft.com", true},
		{"www.nazhisoft.com", ".nazhisoft.com", true},
		{"doc.nazhisoft.com", ".nazhisoft.com", true},
		{"evilnazhisoft.com", "nazhisoft.com", false},
		{"evilnazhisoft.com", ".nazhisoft.com", false},
		{"notnazhisoft.com", "nazhisoft.com", false},
		{"evil.com", "nazhisoft.com", false},
		{"", "nazhisoft.com", false},
	}
	for _, tc := range cases {
		got := hasHostSuffix(tc.host, tc.suffix)
		if got != tc.want {
			t.Errorf("hasHostSuffix(%q, %q)=%v, want %v", tc.host, tc.suffix, got, tc.want)
		}
	}
}
