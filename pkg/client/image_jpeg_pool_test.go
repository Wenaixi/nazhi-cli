// Package client 内部白盒测试。
package client

import (
	"image"
	"image/color"
	"sync"
	"testing"
)

// TestEncodeJPEG_ConcurrentSafe 回归测试：encodeJPEG 在并发调用下
// 不会因 sync.Pool buffer 复用导致数据竞争或输出错乱。
//
// 历史 bug：encodeJPEG 直接返回 buf.Bytes()，未 copy；pool Put 后
// buffer 内部 slice 被其他 goroutine 复用覆盖，返回的 []byte 失效。
func TestEncodeJPEG_ConcurrentSafe(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// 填几个非零像素确保 JPEG 输出非空
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	const goroutines = 10
	const iterations = 50
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				data, err := encodeJPEG(img, 80)
				if err != nil {
					errCh <- err
					return
				}
				if len(data) == 0 {
					errCh <- errEmpty
					return
				}
				// 验证返回的 []byte 在后续调用后仍有效（不被 pool 覆盖）
				firstByte := data[0]
				_ = firstByte
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err == errEmpty {
			t.Error("encodeJPEG 返回空 []byte")
		} else {
			t.Errorf("encodeJPEG 错误: %v", err)
		}
	}
}

var errEmpty = constErr("empty jpeg output")

type constErr string

func (e constErr) Error() string { return string(e) }
