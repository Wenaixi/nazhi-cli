package client

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// TestEncodeJPEG_PanicRecovery_PoolStillUsable 验证 encodeJPEG 内部
// jpeg.Encode panic 后，sync.Pool 中的 buffer 仍可正常使用。
//
// 场景：jpeg.Encode 传入 nil *image.RGBA 时触发 nil deref panic。
// 此时 defer buf.Reset() + jpegBufPool.Put(buf) 仍会执行，
// buf 被归还到 pool 但内容不确定。本测试验证：
//   - panic 不被传播到调用方（测试本身 recover）
//   - pool.Get 后续仍返回可用 buffer
//   - 下一轮 encodeJPEG 调用正常
func TestEncodeJPEG_PanicRecovery_PoolStillUsable(t *testing.T) {
	// 1. 制造 panic：nil image → jpeg.Encode 调 img.Bounds() → nil deref
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("encodeJPEG(nil, ...) 应 panic，但未 panic")
			}
		}()
		_, _ = encodeJPEG(nil, 80)
	}()

	// 2. panic 后 pool 仍能用 Get 拿到有效 buffer
	buf, ok := jpegBufPool.Get().(*bytes.Buffer)
	if !ok {
		t.Fatal("jpegBufPool.Get() 应返回 *bytes.Buffer")
	}
	buf.Reset()
	jpegBufPool.Put(buf)

	// 3. 下一轮 encodeJPEG 调用正常（验证 pool 未被 panic 损坏）
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	data, err := encodeJPEG(img, 80)
	if err != nil {
		t.Fatalf("panic 后 encodeJPEG 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("encodeJPEG 返回空数据")
	}
}
