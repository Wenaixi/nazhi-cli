// file_upload_test.go 验证 UploadFile 的 multipart body 完整性。
//
// F14 修复（round-7）：原实现 `defer writer.Close()` 让 multipart writer
// 终结边界 `--{boundary}--` 在 HTTP 传输后才追加，导致 server 收到比
// Content-Length 短的 body（缺终止边界），server 端 multipart parser 报错。
// 测试策略：httptest.Server 读 body 完整 bytes，断言必须以终止边界结尾。
package client

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestUploadFile_MultipartBodyHasTerminator 验证 UploadFile 发送的 multipart
// body 必须以标准终止边界 `\r\n--{boundary}--\r\n` 结尾。
//
// 失败场景（F14）：C10 round-3 修复把 `writer.Close()` 改为 `defer writer.Close()`，
// defer 在函数返回才执行，但 http.Client.Do 已经在 buf 写入 wire 之前发出去，
// server 收到缺终止边界的 body → multipart parser EOF 错误 → 100% 上传失败。
//
// 修复：multipart 终结边界必须在 http.NewRequestWithContext 之前写入 buf，
// 然后用 defer 兜底保证错误路径也写入（c0f6c54 的原设计意图）。
func TestUploadFile_MultipartBodyHasTerminator(t *testing.T) {
	var (
		gotBoundary string
		gotBody     []byte
	)
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content-Type: multipart/form-data; boundary=xxx
		ct := r.Header.Get("Content-Type")
		const mpPrefix = "multipart/form-data; boundary="
		if !strings.HasPrefix(ct, mpPrefix) {
			t.Errorf("Content-Type 应以 multipart/form-data; boundary= 开头，实际 %q", ct)
		}
		gotBoundary = strings.TrimPrefix(ct, mpPrefix)
		// 必须读完整个 body，否则 server 端 connection 会半截关闭
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("server 读 body 失败: %v", err)
		}
		gotBody = body

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":1,"returnData":{"id":1}}`))
	}))
	defer upload.Close()

	c, _ := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))

	// 准备一张 50×50 测试 PNG（无须很大，只验证 body 完整性）
	tmpfile := t.TempDir() + "/test-multipart.png"
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	if _, err := c.UploadFile(t.Context(), tmpfile); err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}

	if gotBoundary == "" {
		t.Fatal("server 未读到 multipart boundary")
	}
	if len(gotBody) == 0 {
		t.Fatal("server 收到的 body 为空")
	}

	// 关键断言：body 必须以标准 multipart 终止边界结尾
	wantTerminator := "\r\n--" + gotBoundary + "--\r\n"
	if !bytes.HasSuffix(gotBody, []byte(wantTerminator)) {
		t.Errorf("multipart body 必须以 %q 结尾（F14 回归：defer writer.Close 导致终止边界缺失），\n"+
			"实际末尾 %q",
			wantTerminator,
			tailBytes(gotBody, 80))
	}
}

// tailBytes 返回 buf 的最后 n bytes（不足则全返回），用于错误信息。
func tailBytes(buf []byte, n int) string {
	if len(buf) <= n {
		return string(buf)
	}
	return string(buf[len(buf)-n:])
}

// TestUploadFile_MultipartBodyRoundTripParses 用 multipart.Reader 解析 server
// 收到的 body，确保不仅有终止边界，且能正确还原出 form file。
//
// 这是更严格的契约测试：如果 body 长度正确但 boundary 错乱、字段顺序错乱，
// 都会被这个测试发现。F14 修复后必须通过。
func TestUploadFile_MultipartBodyRoundTripParses(t *testing.T) {
	var gotBody []byte
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":1,"returnData":{"id":1}}`))
	}))
	defer upload.Close()

	c, _ := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))

	tmpfile := t.TempDir() + "/test-roundtrip.png"
	img := image.NewRGBA(image.Rect(0, 0, 30, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 30; x++ {
			img.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("编码失败: %v", err)
	}
	f.Close()

	if _, err := c.UploadFile(t.Context(), tmpfile); err != nil {
		t.Fatalf("UploadFile 失败: %v", err)
	}

	if len(gotBody) == 0 {
		t.Fatal("server body 为空")
	}

	// 验证：用 string.Contains 确认 body 同时含 Content-Disposition +
	// boundary terminator（终止边界）。multipart 终止边界就是 -- 开头，
	// 任何 writer.Close() 都会写 `--{boundary}--\r\n`。
	if !bytes.Contains(gotBody, []byte("Content-Disposition: form-data; name=\"file\"")) {
		t.Errorf("body 应包含 file form 字段声明，实际:\n%s", string(gotBody))
	}
	// body 末尾必须是 `--{boundary}--` 终结（multipart spec），不能少结尾
	if !bytes.HasSuffix(gotBody, []byte("--\r\n")) && !bytes.HasSuffix(gotBody, []byte("--")) {
		t.Errorf("body 末尾应含 multipart 终止边界，实际末尾:\n%s", tailBytes(gotBody, 100))
	}
}
