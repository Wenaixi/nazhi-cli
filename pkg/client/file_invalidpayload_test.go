package client

import (
	"context"
	"errors"
	"testing"
)

// TestUploadFile_MissingAttachment_IsInvalidPayload 锁定 FILE-1 修订契约：
// 上传不存在的附件文件 → SDK 返回 ErrInvalidPayload（调用方输入问题），
// CLI 漏斗据此归 400/exit3，而非裸 IO 错误走 500/exit2 被脚本无限重试。
func TestUploadFile_MissingAttachment_IsInvalidPayload(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() 失败: %v", err)
	}
	defer c.Close()

	_, err = c.UploadFile(context.Background(), "nonexistent-file-xyz.tmp")
	if err == nil {
		t.Fatal("上传不存在的文件应报错，实际 nil")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Errorf("本地 IO 错误应包 ErrInvalidPayload，实际 %v", err)
	}
}
