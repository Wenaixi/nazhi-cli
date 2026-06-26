// image_prep_gif_test.go 验证带透明 GIF 在 prepareImageForUpload 后
// 输出非黑底（F11 修复）。
//
// F11 证据：image_prep.go:62-65 特例分支 `if format == "gif" && flattened`
// 做了两件事：(1) imaging.Clone(img) 丢弃透明；(2) flattened=false 跳过
// flattenOnWhite。HasTransparency 对 *image.Paletted 始终返回 true。
// 失败场景：透明 GIF → jpeg.Encode 把透明索引解析为黑色 → 黑底 JPG。
package client

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

// TestPrepareImage_GifTransparentNotBlack 透明 GIF 必须合成到白底。
//
// 测试策略：
//  1. 构造一个透明 GIF（左半透明索引 = 0，右半索引 = 1）
//  2. 调 prepareImageForUpload
//  3. 解码输出的 JPG，断言：左半像素的 R 值远 > 50（不是黑底）
//
// 修复前：flattened=false 跳过 flattenOnWhite → jpeg.Encode 黑底 → 左半 R≈0 → FAIL
// 修复后：flattened=true 走 flattenOnWhite → 白底合成 → 左半 R≈255 → PASS
func TestPrepareImage_GifTransparentNotBlack(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/transparent.gif"

	// 构造带透明的 GIF：
	//   - palette: 索引 0 = 透明（透明色），索引 1 = 不透明红色
	//   - 左半 (0..49) 用索引 0（透明），右半 (50..99) 用索引 1（红）
	//
	// jpeg.Encode 对透明索引（无 alpha）会解析为黑色 (0,0,0)。
	// 因此修复前：左半 = 黑底；修复后：左半 = 白底（与 flattenOnWhite 行为一致）。
	const w, h = 100, 50
	pal := color.Palette{
		color.RGBA{0, 0, 0, 0},     // 索引 0：完全透明（视为黑色前景）
		color.RGBA{255, 0, 0, 255}, // 索引 1：不透明红色
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < 50 {
				img.SetColorIndex(x, y, 0) // 左半透明
			} else {
				img.SetColorIndex(x, y, 1) // 右半红
			}
		}
	}

	f, _ := os.Create(tmpfile)
	if err := gif.Encode(f, img, &gif.Options{NumColors: 2}); err != nil {
		f.Close()
		t.Fatalf("GIF 编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 实际 %s", mime)
	}
	if len(data) == 0 {
		t.Fatal("输出数据为空")
	}

	// 解码输出的 JPG，断言左半（透明区）像素的 R 值远大于黑底阈值
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码输出 JPG 失败: %v", err)
	}
	// 取左半中心点 (25, 25) 的像素
	pixel := decoded.At(25, 25)
	r, g, b, _ := pixel.RGBA()
	r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

	t.Logf("GIF 透明区域中心像素: R=%d G=%d B=%d", r8, g8, b8)

	// 黑底判定：R < 50（修复前会触发）
	if r8 < 50 {
		t.Errorf("F11 回归：GIF 透明区域被处理为黑底, R=%d (期望白底 R≈255)", r8)
	}

	// 进一步保险：右半（红色不透明）应该保持红色
	pixelRight := decoded.At(75, 25)
	rR, gR, _, _ := pixelRight.RGBA()
	rR8, gR8 := uint8(rR>>8), uint8(gR>>8)
	if rR8 < 200 || gR8 > 50 {
		t.Errorf("GIF 不透明红色区域失真: R=%d G=%d (期望 R≥200, G≤50)", rR8, gR8)
	}
}

// TestPrepareImage_GifOpaque 纯不透明 GIF 仍然走 flattenOnWhite 不受影响。
//
// 回归测试：删除 `if format=="gif" && flattened` 特例不能影响不透明 GIF 路径。
// 不透明 GIF 在 gif.Decode 时返回 *image.Paletted，但 hasTransparency
// 对 *image.Paletted 始终返回 true，所以不透明 GIF 实际也会走 flattened 分支。
// 这里只验证不 panic + 输出合法 JPG。
func TestPrepareImage_GifOpaque(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/opaque.gif"

	const w, h = 60, 40
	pal := color.Palette{
		color.RGBA{255, 255, 255, 255},
		color.RGBA{0, 0, 0, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, w, h), pal)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetColorIndex(x, y, uint8((x+y)%2)) // 棋盘格
		}
	}

	f, _ := os.Create(tmpfile)
	if err := gif.Encode(f, img, &gif.Options{NumColors: 2}); err != nil {
		f.Close()
		t.Fatalf("GIF 编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 实际 %s", mime)
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("输出 JPG 无法解码: %v", err)
	}

	// 冗余：检查不透明 GIF 经 flatten 后是否仍是棋盘格（白/黑，不应是单一色）
	decoded, _ := jpeg.Decode(bytes.NewReader(data))
	white := decoded.At(0, 0) // 偶数和索引 (0+0)%2 = 0 = 白
	black := decoded.At(1, 0) // (1+0)%2 = 1 = 黑
	_, _, _, whiteA := white.RGBA()
	_, _, _, blackA := black.RGBA()
	_ = whiteA
	_ = blackA
	// 仅 smoke：不要求严格棋盘（flatten 后 RGB 转换可能有偏差），只要能解码
}

// TestPrepareImage_PngTransparentStillFlattens 回归保险：
// PNG 透明合成白底的现有契约不能被 GIF 修复破坏。
func TestPrepareImage_PngTransparentStillFlattens(t *testing.T) {
	c := internalNewTestClient()
	tmpfile := t.TempDir() + "/transparent.png"

	img := image.NewNRGBA(image.Rect(0, 0, 50, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			img.Set(x, y, color.NRGBA{255, 0, 0, 0}) // 完全透明红
		}
	}
	f, _ := os.Create(tmpfile)
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("PNG 编码失败: %v", err)
	}
	f.Close()

	data, mime, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("期望 mime=image/jpeg, 实际 %s", mime)
	}
	decoded, _ := jpeg.Decode(bytes.NewReader(data))
	// 完全透明 → flattenOnWhite → 白底
	pixel := decoded.At(25, 25)
	r, _, _, _ := pixel.RGBA()
	r8 := uint8(r >> 8)
	if r8 < 200 {
		t.Errorf("PNG 完全透明应 flatten 到白底, R=%d (期望 ≈255)", r8)
	}
}
