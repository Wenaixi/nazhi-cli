package e2e

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
)

func TestE2E_File(t *testing.T) {
	t.Run("upload/mock-encode", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "e2e-upload.jpg")
		if err := writeTinyJPEG(tmp, 16, 16, color.RGBA{R: 120, G: 180, B: 200, A: 255}); err != nil {
			t.Fatalf("writeTinyJPEG: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := mockClient.UploadFile(ctx, tmp)
		if err != nil {
			t.Logf("mock upload 预期可能失败（非真域）: %v", err)
		} else {
			t.Logf("mock upload 成功")
		}
	})

	t.Run("upload+download/live", func(t *testing.T) {
		if !liveUpload || !IsLiveAvailable() {
			t.Skip("live file skipped: need NAZHI_USERNAME/PASSWORD and liveUpload")
		}
		tmp := filepath.Join(t.TempDir(), "e2e-live.jpg")
		if err := writeTinyJPEG(tmp, 16, 16, color.RGBA{R: 200, G: 220, B: 100, A: 255}); err != nil {
			t.Fatalf("writeTinyJPEG: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		res, err := liveClient.UploadFile(ctx, tmp)
		if err != nil {
			t.Fatalf("live UploadFile: %v", err)
		}
		if res == nil || res.AttachmentID <= 0 {
			t.Fatalf("live upload 返回空或 ID 非正: %+v", res)
		}
		t.Logf("live upload ok id=%d", res.AttachmentID)
		dst := filepath.Join(t.TempDir(), "e2e-dl.jpg")
		if err := liveClient.DownloadFile(ctx, res.AttachmentID, dst); err != nil {
			t.Fatalf("live DownloadFile: %v", err)
		}
		info, err := os.Stat(dst)
		if err != nil || info.Size() == 0 {
			t.Fatalf("download 落盘失败: %v size=%d", err, fileSize(info))
		}
		t.Logf("live download ok size=%d", info.Size())
	})

	_ = client.ErrBusinessRejected
}

func writeTinyJPEG(path string, w, h int, c color.RGBA) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
}

func fileSize(info os.FileInfo) int64 {
	if info == nil {
		return -1
	}
	return info.Size()
}
