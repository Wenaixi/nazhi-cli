package client

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDownloadFile_MidStreamFailure_ReturnsErrNetwork 锁定 file.go:435-438 的
// 中途传输失败必须包 ErrNetwork 哨兵。
// 当前裸 fmt.Errorf("写入文件失败: %w", copyErr) 让 errors.Is(err, ErrNetwork) 落空。
//
// 模拟：直接调 writeDownloadToFile + 会中途返回 io.ErrUnexpectedEOF 的 Reader，
// 验证返回错误 errors.Is 命中 ErrNetwork。
func TestDownloadFile_MidStreamFailure_ReturnsErrNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "out.bin")

	src := &failAfterNReader{n: 5, err: io.ErrUnexpectedEOF}

	err := writeDownloadToFile(context.Background(), src, dst)
	if err == nil {
		t.Fatal("want error from mid-stream reader, got nil")
	}
	if !errors.Is(err, ErrNetwork) {
		t.Fatalf("want errors.Is(err, ErrNetwork) = true, got err=%v", err)
	}

	// 验证半成品被删除
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("partial file should be removed, but exists: %v", statErr)
	}
}

type failAfterNReader struct {
	n   int
	err error
	pos int
}

func (r *failAfterNReader) Read(p []byte) (int, error) {
	if r.pos >= r.n {
		return 0, r.err
	}
	if len(p) > 0 && r.pos < r.n {
		p[0] = 'X'
		r.pos++
		if r.pos == r.n {
			return 1, r.err
		}
		return 1, nil
	}
	return 0, io.EOF
}

// TestDownloadFile_ContextCanceledMidStream_PassesThrough 锁定：用户主动取消 ctx
// 不应被误标为 ErrNetwork（避免自动重试逻辑误触发）。
func TestDownloadFile_ContextCanceledMidStream_PassesThrough(t *testing.T) {
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "out.bin")

	ctx, cancel := context.WithCancel(context.Background())
	src := &cancellingReader{cancel: cancel}

	err := writeDownloadToFile(ctx, src, dst)
	if err == nil {
		t.Fatal("want error from cancelled ctx, got nil")
	}
	if errors.Is(err, ErrNetwork) {
		t.Fatalf("ctx-cancellation must NOT be classified as ErrNetwork, got %v", err)
	}
}

type cancellingReader struct {
	cancel context.CancelFunc
	done   bool
}

func (r *cancellingReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		go r.cancel()
		time.Sleep(20 * time.Millisecond)
		return 1, nil
	}
	return 0, context.Canceled
}
