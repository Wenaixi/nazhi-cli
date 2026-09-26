// file_download_oversize_test.go — ：DownloadFile 流式写
// 无字节上限的防线补丁。
//
// 背景：writeDownloadToFile 的 copyCtx 对上游流式写入无任何字节上限，攻击者
// 借重定向到受信子域的无限流可把磁盘写满（0 字节已归 ErrInvalidResponse，
// 但超大流未防；与同文件附件 64KB / UploadFile 20MB 纪律不对称）。
// 修复：copyCtx 内嵌 LimitReader 超限判定，超限删半成品归 ErrInvalidResponse。
package client

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteDownloadToFile_RejectsOversizedStream 锁定：超过 maxDownloadBytes
// 的流式响应在写满前被拦截、删除半成品、归 ErrInvalidResponse（不可重试语义，
// 与 0 字节同族——无限流是永久性条件而非瞬时网络故障）。旧实现把整个流
// 写完才返回，无上限。
func TestWriteDownloadToFile_RejectsOversizedStream(t *testing.T) {
	// 构造一个超过上限（50MB+1）的流：不用真实写 50MB 内存，用无限/超长
	// Reader + 上限判定应在其超出前截断。这里用 50MB 真实流验证边界。
	oversize := bytes.NewReader(make([]byte, maxDownloadBytes+1))
	dst := filepath.Join(t.TempDir(), "oversize.bin")

	err := writeDownloadToFile(context.Background(), oversize, dst)
	if err == nil {
		t.Fatal("超大流应被拒绝，实际 nil")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("超大流应归 ErrInvalidResponse（不可重试），实际 %v", err)
	}
	// 半成品必须删除（Windows 下先 Close 再 Remove 已由 writeDownloadToFile 保证）
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("超大流拒绝后应删除半成品，实际仍存在: %v", statErr)
	}
}
