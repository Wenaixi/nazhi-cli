package client

import (
	"image"
	"image/color"
	"testing"
)

// TestHasTransparency_NYCbCrA 有损 WebP（VP8+ALPH）解码产物是 *image.NYCbCrA，
// 必须被识别为含透明通道并走白底合成；否则 jpeg.Encode 丢弃 alpha，
// 全透明像素经 color.NYCbCrA.RGBA() 预乘后落为黑色背景。
// 注意：无损 VP8L WebP 解码产物是 *image.NRGBA，已被既有分支覆盖，
// 本测试针对的是 golang.org/x/image/webp decode.go:96 的 NYCbCrA 构造点。
func TestHasTransparency_NYCbCrA(t *testing.T) {
	img := image.NewNYCbCrA(image.Rect(0, 0, 8, 8), image.YCbCrSubsampleRatio420)
	if !hasTransparency(img) {
		t.Fatal("*image.NYCbCrA 应被判定为含透明通道（当前缺失该分支会导致透明区编码后变黑底）")
	}
}

// TestFlattenOnWhite_NYCbCrA NYCbCrA 全透明区域合成后必须落白底而非黑色。
// 直接验证 flattenOnWhite 的输出像素，绕过 JPEG 编码环节隔离缺陷。
func TestFlattenOnWhite_NYCbCrA(t *testing.T) {
	const w, h = 8, 8
	img := image.NewNYCbCrA(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	// YCbCr 填充为纯红（Y≈76, Cb≈84, Cr≈255），alpha 全 0（完全透明）
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.YCbCr.Y[y*img.YStride+x] = 76
			ci := (y/2)*img.CStride + x/2
			img.YCbCr.Cb[ci] = 84
			img.YCbCr.Cr[ci] = 255
		}
	}
	flat, ok := flattenOnWhite(img).(*image.RGBA)
	if !ok {
		t.Fatalf("flattenOnWhite 应返回 *image.RGBA")
	}
	r, g, b, _ := flat.At(4, 4).RGBA()
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Fatalf("全透明 NYCbCrA 合成后期望白底, 实际 R=%d G=%d B=%d", r>>8, g>>8, b>>8)
	}
	_ = color.White
}
