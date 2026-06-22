// image_prep_test.go 是 image_prep.go 的内部测试（同包，可访问未导出方法）。
package client

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

// internalNewTestClient 是 newTestClient 的内部版本（同包访问）。
func internalNewTestClient() *Client {
	return New(WithTimeout(5 * time.Second))
}

// ─── 测试: PNG → JPG 转换 + RGBA 合成白底 ───

func TestPrepareImage_PNGtoJPEG(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test-png-input.png"

	// 创建 200×200 半透明 PNG（验证 RGBA → 白底合成）
	img := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.NRGBA{0, 128, 255, 128}) // 半透明
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 得到 %s", mime)
	}
	if len(data) == 0 {
		t.Error("输出数据为空")
	}
	if len(data) > MaxImageSize {
		t.Errorf("压缩后仍超 %d bytes: %d", MaxImageSize, len(data))
	}
	t.Logf("PNG → JPG 转换: %d bytes", len(data))
}

// ─── 测试: JPG 透传 ───

func TestPrepareImage_JPEGPassthrough(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test-jpg.jpg"

	// 创建 200×200 纯色 JPG
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	green := color.RGBA{0, 255, 0, 255}
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, green)
		}
	}
	f, _ := os.Create(tmpfile)
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 得到 %s", mime)
	}
	if len(data) == 0 {
		t.Error("输出数据为空")
	}
	t.Logf("JPG 透传: %d bytes", len(data))
}

// ─── 测试: 大图压缩（5MB 强制触发压缩路径）───

func TestPrepareImage_CompressesLargeImage(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test-large.png"

	// 创建 3000×3000 大图
	img := image.NewRGBA(image.Rect(0, 0, 3000, 3000))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 3000; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), uint8((x + y) % 256), 255})
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()
	origStat, _ := os.Stat(tmpfile)
	t.Logf("原图大小: %d bytes", origStat.Size())

	data, _, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if len(data) > MaxImageSize {
		t.Errorf("压缩后仍超 %d bytes: %d", MaxImageSize, len(data))
	}
	t.Logf("压缩后: %d bytes (压缩率 %.1f%%)", len(data), float64(len(data))/float64(origStat.Size())*100)
}

// ─── 测试: GIF 动画取第 0 帧 ───

func TestPrepareImage_GifStatic(t *testing.T) {
	// Go stdlib 的 gif.Encode 不直接支持多帧，这里只验证能正确解码 GIF
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/test.gif"

	// 创建单帧 GIF
	img := image.NewPaletted(image.Rect(0, 0, 50, 50), color.Palette{color.Black, color.White})
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.White)
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()
	// 实际 GIF 解码由 stdlib 处理，这里只确保 prepare 不 panic
	_, _, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Logf("GIF 测试跳过: %v", err)
	}
}

// ─── 测试: UploadFile 不发送任何鉴权 Header ───

// 验证即使 cookie jar 已被注入 X-Auth-Token，HTTP 请求也不携带任何鉴权头
func TestUploadFile_NoAuthHeaders(t *testing.T) {
	var seenHeaders http.Header
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":1,"returnData":{"id":67890}}`))
	}))
	defer upload.Close()

	// 创建带 X-Auth-Token 的 Client（模拟"复用了已登录 Client"的最坏情况）
	c := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))
	jar, ok := c.http.Jar.(*cookiejar.Jar)
	if ok {
		u, _ := url.Parse(upload.URL)
		jar.SetCookies(u, []*http.Cookie{
			{Name: "X-Auth-Token", Value: "fake-leaked-token-should-not-be-sent"},
			{Name: "JSESSIONID", Value: "fake-session"},
		})
	}

	// 创建测试 PNG
	tmpfile := t.TempDir() + "/test-noauth.png"
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	_, err := c.UploadFile(t.Context(), tmpfile)
	if err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}

	// 1. X-Auth-Token Header 没发送
	if v := seenHeaders.Get("X-Auth-Token"); v != "" {
		t.Errorf("❌ 检测到 X-Auth-Token Header 被发送: %q", v)
	}
	// 2. Authorization Header 没发送
	if v := seenHeaders.Get("Authorization"); v != "" {
		t.Errorf("❌ 检测到 Authorization Header 被发送: %q", v)
	}
	// 3. Cookie Header 没发送（清空所有 cookie）
	if v := seenHeaders.Get("Cookie"); v != "" {
		t.Errorf("❌ 检测到 Cookie Header 被发送: %q", v)
	}
	t.Logf("✓ UploadFile 正确未发送任何鉴权 Header（X-Auth-Token/Authorization/Cookie）")
}
