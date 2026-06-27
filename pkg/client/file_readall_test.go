// file_readall_test.go 验证 UploadFile 正确处理 io.ReadAll 错误
// (修复前 readall 错误被 _ 吞噬，导致 json.Unmarshal 误报 EOF)。
package client_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// makeJpegTempFile 在 t.TempDir() 中创建一个合法的 JPEG 测试文件，
// 并返回其绝对路径。文件大小约几 KB，远低于 MaxImageSize。
func makeJpegTempFile(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	// 一次性填充: 给每个像素一个非零颜色避免压缩到极小
	for i := 0; i < 32*32; i++ {
		x := i % 32
		y := i / 32
		img.Set(x, y, color.RGBA{byte(x * 8), byte(y * 8), 128, 255})
	}
	path := t.TempDir() + "/test.jpg"
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("编码 jpeg 失败: %v", err)
	}
	return path
}

// brokenUploadServer 构造一个故意在 Write headers 之后关闭连接的
// httptest server，模拟服务端在响应体未完全写出时断网。
// 这种场景下客户端的 io.ReadAll 会返回非 nil error (connection reset
// by peer / unexpected EOF)，用来验证修复前 _ 吞噬错误的 bug。
func brokenUploadServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 失败: %v", err)
	}
	addr := listener.Addr().String()
	srv := &httptest.Server{
		Listener: listener,
		Config: &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 写完 headers 立刻 hijack + Close, 不写 body
				hj, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "no hijack", 500)
					return
				}
				conn, _, err := hj.Hijack()
				if err != nil {
					return
				}
				// 写一个最小的 HTTP/1.1 响应: headers + Content-Length: 100 但 body 长度 0
				// 这样客户端 ReadAll 会先读到 EOF 拿到 0 字节, 但根本原因是连接被关
				header := "HTTP/1.1 200 OK\r\n" +
					"Content-Type: application/json\r\n" +
					"Content-Length: 100\r\n" +
					"\r\n"
				if _, err := conn.Write([]byte(header)); err != nil {
					_ = conn.Close()
					return
				}
				// 不写 body, 直接关连接
				_ = conn.Close()
			}),
			ReadHeaderTimeout: 2 * time.Second,
		},
	}
	srv.Start()
	return srv, "http://" + addr
}

// TestUploadFile_ReadAllErrorIsWrappedAsErrNetwork 回归测试:
// UploadFile 在 io.ReadAll 失败时必须返回包装了 ErrNetwork 的错误。
// 修复前 readall 错误被 _ 吞噬, 客户端会误报 "解析上传响应失败: EOF"，
// 丢失网络中断的根因。
func TestUploadFile_ReadAllErrorIsWrappedAsErrNetwork(t *testing.T) {
	brokenSrv, brokenURL := brokenUploadServer(t)
	defer brokenSrv.Close()

	// 把 upload URL 指到断网 server (注意必须用 url.Parse 校验以防开发机没装到 path)
	if _, err := url.Parse(brokenURL); err != nil {
		t.Fatalf("brokenURL 无效: %v", err)
	}

	jpegPath := makeJpegTempFile(t)

	c, _ := client.New(
		client.WithUploadURL(brokenURL),
		client.WithTimeout(3*time.Second),
	)

	_, err := c.UploadFile(context.Background(), jpegPath)
	if err == nil {
		t.Fatal("UploadFile 应返回错误, 但成功了")
	}

	// 错误信息必须明确说明「读取响应体失败」并包含 ErrNetwork 哨兵
	if !strings.Contains(err.Error(), "读取上传响应体失败") {
		t.Errorf("错误信息应包含「读取上传响应体失败」, 实际: %v", err)
	}
	if !errors.Is(err, client.ErrNetwork) {
		t.Errorf("错误应包装 ErrNetwork 哨兵, 实际: %v", err)
	}
}

// TestUploadFile_ReadAllErrorNotSwallowed 防护测试:
// 错误信息绝不能是含糊的 EOF 误报 (修复前的症状)。
// 因为 json.Unmarshal([]) 在 body 为空时返回 EOF, 旧实现会报「解析上传响应失败: EOF」,
// 把网络层根因丢失, 误导用户以为是协议错误。
func TestUploadFile_ReadAllErrorNotSwallowed(t *testing.T) {
	brokenSrv, brokenURL := brokenUploadServer(t)
	defer brokenSrv.Close()

	jpegPath := makeJpegTempFile(t)

	c, _ := client.New(
		client.WithUploadURL(brokenURL),
		client.WithTimeout(3*time.Second),
	)

	_, err := c.UploadFile(context.Background(), jpegPath)
	if err == nil {
		t.Fatal("UploadFile 应返回错误")
	}
	if strings.HasPrefix(err.Error(), "解析上传响应失败: EOF") {
		t.Errorf("修复失败: 错误信息仍是「解析上传响应失败: EOF」(readall 错误被吞噬), 实际: %v", err)
	}
}

// 防止引入未使用的 bytes 包 (内部 helper 已使用 image/jpeg)
var _ = bytes.NewReader
