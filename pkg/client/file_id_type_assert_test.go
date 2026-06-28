// file_id_type_assert_test.go 验证 UploadFile 解析 returnData['id'] 失败时
// 错误信息能区分「字段缺失」与「类型不匹配」两种根因。
// 修复契约：原实现 `id, ok := result["id"].(float64); if !ok { 报"缺少 id 字段" }`
// 把两种语义完全不同的根因合并成同一条错误信息。修复后：
// - 字段不存在 → "returnData 中缺少 id 字段"
// - 字段存在但类型不匹配 → "returnData.id 类型不匹配，期望 float64 实际 %T"
package client_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

// makeJpegTempFileForID 在 t.TempDir() 中创建一个合法的 JPEG 测试文件。
func makeJpegTempFileForID(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
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

// uploadServerWithReturnData 构造一个返回指定 returnData 的上传 mock server。
func uploadServerWithReturnData(t *testing.T, returnData string) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// code=1 表示业务成功，returnData 由测试参数控制
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"code":1,"msg":"ok","returnData":%s}`, returnData)
	}))
	return srv, srv.URL
}

// TestUploadFile_IdFieldMissing 验证 returnData 缺少 id 字段时，
// 错误信息明确说明「缺少 id 字段」（而非含糊的类型不匹配）。
func TestUploadFile_IdFieldMissing(t *testing.T) {
	srv, srvURL := uploadServerWithReturnData(t, `{"name":"test.png"}`) // 无 id 字段
	defer srv.Close()

	c, _ := client.New(
		client.WithUploadURL(srvURL),
		client.WithTimeout(3*time.Second),
	)

	jpegPath := makeJpegTempFileForID(t)
	_, err := c.UploadFile(context.Background(), jpegPath)
	if err == nil {
		t.Fatal("UploadFile 应返回错误")
	}

	// 必须明确说「缺少 id 字段」
	if !strings.Contains(err.Error(), "缺少") || !strings.Contains(err.Error(), "id") {
		t.Errorf("错误信息应说明「缺少 id 字段」, 实际: %v", err)
	}
	// 必须包装 ErrUploadRejected 哨兵
	if !errors.Is(err, client.ErrUploadRejected) {
		t.Errorf("错误应包装 ErrUploadRejected, 实际: %v", err)
	}
}

// TestUploadFile_IdTypeMismatch 验证 returnData.id 是 string 类型时，
// 错误信息明确说明「类型不匹配」+ 期望/实际类型（而非含糊的「缺少」）。
func TestUploadFile_IdTypeMismatch(t *testing.T) {
	// id 字段存在但类型是 string（实际服务端极少见, 但防御性必须有清晰诊断）
	srv, srvURL := uploadServerWithReturnData(t, `{"id":"abc123"}`)
	defer srv.Close()

	c, _ := client.New(
		client.WithUploadURL(srvURL),
		client.WithTimeout(3*time.Second),
	)

	jpegPath := makeJpegTempFileForID(t)
	_, err := c.UploadFile(context.Background(), jpegPath)
	if err == nil {
		t.Fatal("UploadFile 应返回错误")
	}

	errMsg := err.Error()

	// 必须明确说「类型不匹配」（区分于「字段缺失」）
	if !strings.Contains(errMsg, "类型不匹配") && !strings.Contains(errMsg, "类型") {
		t.Errorf("错误信息应说明「类型不匹配」, 实际: %v", err)
	}
	// 必须包含实际类型信息
	if !strings.Contains(errMsg, "string") {
		t.Errorf("错误信息应包含实际类型 string（%T）, 实际: %v", err, err)
	}
	// 不应该错误地报「缺少 id 字段」（字段是存在的，只是类型不对）
	if strings.Contains(errMsg, "缺少 id 字段") {
		t.Errorf("错误信息不应报「缺少 id 字段」（字段存在）, 实际: %v", err)
	}
	// 必须包装 ErrUploadRejected
	if !errors.Is(err, client.ErrUploadRejected) {
		t.Errorf("错误应包装 ErrUploadRejected, 实际: %v", err)
	}
}

// TestUploadFile_IdIsNull 验证 returnData.id 是 null 时的诊断信息。
func TestUploadFile_IdIsNull(t *testing.T) {
	srv, srvURL := uploadServerWithReturnData(t, `{"id":null}`)
	defer srv.Close()

	c, _ := client.New(
		client.WithUploadURL(srvURL),
		client.WithTimeout(3*time.Second),
	)

	jpegPath := makeJpegTempFileForID(t)
	_, err := c.UploadFile(context.Background(), jpegPath)
	if err == nil {
		t.Fatal("UploadFile 应返回错误")
	}

	// null 字段应被 case nil 分支识别，错误信息明确说「null」
	errMsg := err.Error()
	if !strings.Contains(errMsg, "null") {
		t.Errorf("id=null 错误信息应说明字段为 null, 实际: %v", err)
	}
}
