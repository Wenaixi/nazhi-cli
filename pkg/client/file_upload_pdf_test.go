// file_upload_pdf_test.go 锁死 PDF 直传行为：白名单命中、字节与文件名原样、
// 超限不发请求。PDF 是官方 tip 之外唯一放行的扩展名（用户明确要求）。
package client

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestUploadFile_PDFDirectUpload 验证 .pdf 走非图片直传路径：字节不改写、
// 文件名保留原扩展名（不转 JPG）。
func TestUploadFile_PDFDirectUpload(t *testing.T) {
	wantData := []byte("%PDF-1.4 测试 PDF 内容")
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
		_, _ = w.Write([]byte(`{"code":1,"returnData":{"id":3,"name":"report.pdf"}}`))
	}))
	defer upload.Close()

	path := t.TempDir() + "/report.pdf"
	if err := os.WriteFile(path, wantData, 0o600); err != nil {
		t.Fatalf("写入测试 PDF 失败: %v", err)
	}

	c, err := New(WithUploadURL(upload.URL), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("构造 Client 失败: %v", err)
	}
	defer c.Close()

	result, err := c.UploadFile(t.Context(), path)
	if err != nil {
		t.Fatalf("UploadFile 不应拒绝 PDF: %v", err)
	}
	if result == nil || result.AttachmentID != 3 || result.AttachmentName != "report.pdf" {
		t.Fatalf("上传结果错误: %+v", result)
	}
	if gotName != "report.pdf" {
		t.Fatalf("PDF 应保留原文件名，实际 %q", gotName)
	}
	if !bytes.Equal(gotData, wantData) {
		t.Fatalf("PDF 内容被改写: want=%q got=%q", wantData, gotData)
	}
}

// TestUploadFile_PDFRejectsOversizeBeforeRequest 验证超 2MB 的 PDF 在本地
// 拒绝且不发 HTTP 请求。
func TestUploadFile_PDFRejectsOversizeBeforeRequest(t *testing.T) {
	var requests int
	upload := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upload.Close()

	path := t.TempDir() + "/too-large.pdf"
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
		t.Fatalf("超限 PDF 应返回 ErrFileTooLarge，实际: %v", err)
	}
	if requests != 0 {
		t.Fatalf("超限 PDF 不应发出 HTTP 请求，实际 %d 次", requests)
	}
}
