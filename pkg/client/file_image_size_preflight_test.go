package client

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestUploadFile_OversizedImage_RejectedBeforeDecode 锁定防御纵深契约：
// 图片分支与附件分支同样必须在解码（全量读入内存）之前做体积预检——
// 高压缩比畸形大图可在解码阶段放大数十倍内存；超限直接 ErrFileTooLarge，
// 不进入解码管线。
func TestUploadFile_OversizedImage_RejectedBeforeDecode(t *testing.T) {
	c, err := New(WithUploadURL("http://unused.invalid"))
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}

	path := filepath.Join(t.TempDir(), "huge.jpg")
	// 写一个超过 MaxImageDecodePreflight 的伪 jpg 文件（内容无所谓，预检先于解码）
	if err := os.WriteFile(path, make([]byte, maxImageDecodePreflight+1), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = c.UploadFile(context.Background(), path)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("超大图片应在解码前被拒绝并归 ErrFileTooLarge，实际: %v", err)
	}
}
