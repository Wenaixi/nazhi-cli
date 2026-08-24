// scan_test.go 验证病毒扫描注入点的 fail-closed 契约与咽喉点位置。
package client

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeScanner 可编程的内存扫描器。
type fakeScanner struct {
	err error // 非 nil 时 ScanUpload 返回该错误
}

func (f *fakeScanner) ScanUpload(ctx context.Context, data []byte) error {
	return f.err
}

// newTestClient 构造指向 httptest 上传服务的 Client，附带一张最小 PNG。
func newTestClient(t *testing.T, hit *int) (*Client, string) {
	t.Helper()
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hit != nil {
			*hit++
		}
		drainAndClose(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":9}}`))
	}))
	t.Cleanup(upload.Close)

	c, err := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("构造 Client 失败: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	path := filepath.Join(t.TempDir(), "pic.png")
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{255, 255, 255, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建测试图片失败: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码测试图片失败: %v", err)
	}
	f.Close()
	return c, path
}

// TestUploadFile_ScanCleanAllowsUpload 干净文件正常上传。
func TestUploadFile_ScanCleanAllowsUpload(t *testing.T) {
	var hits int
	c, path := newTestClient(t, &hits)
	c.uploadScanner = &fakeScanner{}

	if _, err := c.UploadFile(t.Context(), path); err != nil {
		t.Fatalf("干净文件应上传成功: %v", err)
	}
	if hits != 1 {
		t.Fatalf("应发出 1 次上传请求，实际 %d", hits)
	}
}

// TestUploadFile_ScanInfectedRejectsBeforeRequest 检出病毒：本地拒绝且不发请求。
func TestUploadFile_ScanInfectedRejectsBeforeRequest(t *testing.T) {
	var hits int
	c, path := newTestClient(t, &hits)
	c.uploadScanner = &fakeScanner{err: ErrVirusDetected}

	_, err := c.UploadFile(t.Context(), path)
	if !errors.Is(err, ErrVirusDetected) {
		t.Fatalf("检出病毒应返回 ErrVirusDetected，实际: %v", err)
	}
	if hits != 0 {
		t.Fatalf("感染文件不应触达网络，实际 %d 次请求", hits)
	}
}

// TestUploadFile_ScanErrorFailClosed 扫描服务故障：fail-closed 拒绝且不发请求。
// 这是安全底线——「判定未知」绝不等于「干净」。
func TestUploadFile_ScanErrorFailClosed(t *testing.T) {
	var hits int
	c, path := newTestClient(t, &hits)
	c.uploadScanner = &fakeScanner{err: ErrScanUnavailable}

	_, err := c.UploadFile(t.Context(), path)
	if !errors.Is(err, ErrScanUnavailable) {
		t.Fatalf("扫描失败应返回 ErrScanUnavailable，实际: %v", err)
	}
	if hits != 0 {
		t.Fatalf("扫描失败时不应发上传请求（fail-closed），实际 %d 次", hits)
	}
}

// TestWithClamavScanner_RejectsEmptyAddr 空 addr 不 panic、不设置扫描器。
func TestWithClamavScanner_RejectsEmptyAddr(t *testing.T) {
	c, _ := newTestClient(t, nil)
	WithClamavScanner("")(c)
	if c.uploadScanner != nil {
		t.Fatal("空 addr 不应设置扫描器")
	}
}

// TestWithClamavScanner_BadAddrWarnsAndSkips 不可达地址构造失败时保持无扫描器。
func TestWithClamavScanner_BadAddrWarnsAndSkips(t *testing.T) {
	c, _ := newTestClient(t, nil)
	// go-clamav 对非法 addr 在 New 阶段报错——守卫应吞掉并保持 nil。
	WithClamavScanner("not-a-valid-addr")(c)
	if c.uploadScanner != nil {
		t.Fatal("构造失败不应设置扫描器")
	}
}
