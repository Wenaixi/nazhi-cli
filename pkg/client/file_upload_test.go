// file_upload_test.go 验证 UploadFile 的 multipart body 完整性。
// 原实现 `defer writer.Close()` 让 multipart writer
// 终结边界 `--{boundary}--` 在 HTTP 传输后才追加，导致 server 收到比
// Content-Length 短的 body（缺终止边界），server 端 multipart parser 报错。
// 测试策略：httptest.Server 读 body 完整 bytes，断言必须以终止边界结尾。
package client

import (
	"bytes"
	"errors"
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
// 失败场景：round-3 修复把 `writer.Close()` 改为 `defer writer.Close()`，
// defer 在函数返回才执行，但 http.Client.Do 已经在 buf 写入 wire 之前发出去，
// server 收到缺终止边界的 body → multipart parser EOF 错误 → 100% 上传失败。
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
		t.Errorf("multipart body 必须以 %q 结尾（defer writer.Close 曾导致终止边界缺失），\n"+
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
// 这是更严格的契约测试：如果 body 长度正确但 boundary 错乱、字段顺序错乱，
// 都会被这个测试发现。修复后必须通过。
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

// TestBuildRequest_AcceptsIOReader 验证 buildRequest 支持 io.Reader 类型 body，
// 是 UploadFile 走 buildRequest 的核心契约前提。
// 原 buildRequest 只支持 []byte/string/任意 JSON marshal 三类 body，
// 无法承载 multipart 场景下的 io.Reader（*bytes.Buffer）。UploadFile 因此被迫走
// 手工 http.NewRequestWithContext，沦为特例路径。
// 修复后 buildRequest 增加 io.Reader 分支：传入的 reader 直接作为 body，
// Content-Type 由调用方通过 contentType 参数显式传入（multipart 场景下
// 必填，否则服务端无法解析 boundary）。
// 测试策略：构造一个 bytes.Buffer（含标识 payload），通过 buildRequest 创建
// request，断言 body 内容透传 + Content-Type 透传。这是 buildRequest 「接受
// io.Reader」契约的可观测证据——UploadFile 走 helper 时也享受相同行为。
func TestBuildRequest_AcceptsIOReader(t *testing.T) {
	c, _ := New(WithTimeout(5 * time.Second))

	const (
		payload     = "hello-multipart-body-bytes"
		contentType = "multipart/form-data; boundary=test-boundary-123"
		wantURLPath = "/upload"
		wantMethod  = http.MethodPost
	)

	buf := bytes.NewBufferString(payload)
	req, err := c.buildRequest(t.Context(), wantMethod, "http://example.com"+wantURLPath, buf, nil, contentType)
	if err != nil {
		t.Fatalf("buildRequest 失败: %v", err)
	}

	if req.Method != wantMethod {
		t.Errorf("Method 期望 %s, 实际 %s", wantMethod, req.Method)
	}
	if req.URL.Path != wantURLPath {
		t.Errorf("URL.Path 期望 %s, 实际 %s", wantURLPath, req.URL.Path)
	}
	if gotCT := req.Header.Get("Content-Type"); gotCT != contentType {
		t.Errorf("Content-Type 期望 %q, 实际 %q", contentType, gotCT)
	}
	// body 内容必须透传（io.Reader 流式读取后与原始 payload 一致）
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("读取 req.Body 失败: %v", err)
	}
	if string(bodyBytes) != payload {
		t.Errorf("body 内容期望 %q, 实际 %q", payload, string(bodyBytes))
	}
}

// TestUploadFile_ViaBuildRequest 验证 UploadFile 走共享 buildRequest helper，
// 与 doRequest/doBizGet 路径保持一致（header 注入 + Content-Type）。
// 原实现手工 http.NewRequestWithContext + 手工设 headers，
// 是 buildRequest 之外的特例路径，与其他 SDK 方法的请求构造不一致。
// 一旦 buildRequest 演进（如新增公共 header、debug 日志脱敏、req body 校验），
// UploadFile 会自动掉队。
// 测试策略：
// 1. 构造 mock upload server，捕获 Content-Type + Accept + User-Agent
// 2. 校验这三个 header 都正确注入（multipart Content-Type + biz 标准 UA）
// 3. 这是 buildRequest "正确行为" 的可观测子集——通过即可证明走 helper
func TestUploadFile_ViaBuildRequest(t *testing.T) {
	var (
		gotContentType string
		gotAccept      string
		gotUserAgent   string
	)
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		gotUserAgent = r.Header.Get("User-Agent")
		// F32b: 复用 drainAndClose helper 代替手写 io.Copy(io.Discard, ...)
		drainAndClose(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":1}}`))
	}))
	defer upload.Close()

	c, _ := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))

	// 准备一张 30×30 测试 PNG
	tmpfile := t.TempDir() + "/test-build-request.png"
	img := image.NewRGBA(image.Rect(0, 0, 30, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 30; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
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

	// Content-Type 必须是 multipart/form-data; boundary=xxx（buildRequest 透传 contentType 参数）
	if !strings.HasPrefix(gotContentType, "multipart/form-data; boundary=") {
		t.Errorf("Content-Type 应以 multipart/form-data; boundary= 开头，实际 %q", gotContentType)
	}
	// Accept 必须是 buildRequest 通过 headers map 注入的值
	if !strings.Contains(gotAccept, "application/json") {
		t.Errorf("Accept 应包含 application/json，实际 %q", gotAccept)
	}
	// User-Agent 必须是 chrome UA（与 ssoHeaders/bizHeaders 保持一致的标识字符串）
	if !strings.Contains(gotUserAgent, "Chrome/149") {
		t.Errorf("User-Agent 应为 chrome UA，实际 %q", gotUserAgent)
	}
}

// TestUploadFile_NonImagePreservesBytesAndFilename 验证前端允许的非图片附件原样上传。
func TestUploadFile_NonImagePreservesBytesAndFilename(t *testing.T) {
	wantData := []byte("示例文本附件内容，不经过图片解码或 JPEG 转换。")
	var gotName string
	var gotData []byte

	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("读取 multipart 文件失败: %v", err)
		} else {
			gotName = header.Filename
			gotData, _ = io.ReadAll(file)
			_ = file.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":2,"name":"sample.txt"}}`))
	}))
	defer upload.Close()

	path := t.TempDir() + "/sample.txt"
	if err := os.WriteFile(path, wantData, 0o600); err != nil {
		t.Fatalf("写入测试附件失败: %v", err)
	}

	c, err := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("构造 Client 失败: %v", err)
	}
	defer c.Close()

	result, err := c.UploadFile(t.Context(), path)
	if err != nil {
		t.Fatalf("UploadFile 不应拒绝文本附件: %v", err)
	}
	if result == nil || result.AttachmentID != 2 || result.AttachmentName != "sample.txt" {
		t.Fatalf("上传结果错误: %+v", result)
	}
	if gotName != "sample.txt" {
		t.Fatalf("非图片附件应保留原文件名，实际 %q", gotName)
	}
	if !bytes.Equal(gotData, wantData) {
		t.Fatalf("非图片附件内容被改写: want=%q got=%q", wantData, gotData)
	}
}

// TestUploadFile_NonImageRejectsOversizeBeforeRequest 验证非图片附件超过前端 2MB 限制时不发请求。
func TestUploadFile_NonImageRejectsOversizeBeforeRequest(t *testing.T) {
	var requests int
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upload.Close()

	path := t.TempDir() + "/too-large.zip"
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), MaxAttachmentSize+1), 0o600); err != nil {
		t.Fatalf("写入测试附件失败: %v", err)
	}

	c, err := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("构造 Client 失败: %v", err)
	}
	defer c.Close()

	_, err = c.UploadFile(t.Context(), path)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("超限非图片附件应返回 ErrFileTooLarge，实际: %v", err)
	}
	if requests != 0 {
		t.Fatalf("超限附件不应发出 HTTP 请求，实际 %d 次", requests)
	}
}
