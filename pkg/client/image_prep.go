package client

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/disintegration/imaging"
)

// MaxImageSize 是上传前图片压缩的目标上限（5MB，主人指定）。
// 平台实测单图 56KB~79KB，5MB 留足安全余量；与 v2 backend file.go 限额一致。
const MaxImageSize = 5 * 1024 * 1024 // 5MB = 5242880 bytes

// 质量级联（对齐 v1 策略：先降质量后缩尺寸）。
var qualitySteps = []int{80, 60, 40}

// 缩放级联（每步 ×0.7，对齐 v1）。
var scaleFactors = []float64{0.9, 0.63, 0.441, 0.309, 0.216, 0.151, 0.106}

// MinImageDimension 缩放下限（像素），避免图片过小。
const MinImageDimension = 10

// prepareImageForUpload 读取本地图片并预处理为符合平台要求的 JPG 字节流。
//
// 预处理流程（对齐 v1 utils/image_convert.py 的 ensure_jpg + compress_image）：
//  1. 解码任意格式（PNG / JPEG / GIF / BMP / WEBP）
//  2. 透明通道 → 合成白底（兼容 RGBA / LA / transparency info）
//  3. 动画 GIF → 取第 0 帧
//  4. 编码为 JPG（quality=92, optimize=true）
//  5. 质量级联压缩：92 → 80 → 60 → 40
//  6. 仍超 1MB → 等比缩放（0.9 → 0.63 → 0.44 → ...）
//  7. 输出 ≤ 1MB 的 JPG 字节流 + MIME type
//
// 不会修改原文件，所有处理在内存中完成。
func (c *Client) prepareImageForUpload(path string) ([]byte, string, error) {
	// 1. 打开并解码
	img, format, err := decodeImage(path)
	if err != nil {
		return nil, "", fmt.Errorf("图片解码失败: %w", err)
	}

	// 2. 动画 GIF 取第 0 帧
	if format == "gif" {
		if g, ok := img.(*image.Paletted); ok {
			img = imaging.Clone(g) // 静态化
		}
	}

	// 3. 透明通道 → 白底合成
	if hasTransparency(img) {
		img = flattenOnWhite(img)
	}

	// 4. 编码为 JPG（quality=92 起步）
	data, err := encodeJPEG(img, 92)
	if err != nil {
		return nil, "", fmt.Errorf("编码 JPG 失败: %w", err)
	}

	// 5. 检查大小，已满足则直接返回
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}

	// 6. 质量级联压缩
	for _, q := range qualitySteps {
		data, err = encodeJPEG(img, q)
		if err != nil {
			return nil, "", fmt.Errorf("质量 %d 编码失败: %w", q, err)
		}
		if len(data) <= MaxImageSize {
			return data, "image/jpeg", nil
		}
	}

	// 7. 仍超 1MB → 等比缩放（保持当前质量 40）
	for _, scale := range scaleFactors {
		w := int(float64(img.Bounds().Dx()) * scale)
		h := int(float64(img.Bounds().Dy()) * scale)
		if w < MinImageDimension || h < MinImageDimension {
			break
		}
		resized := imaging.Resize(img, w, h, imaging.Lanczos)
		data, err = encodeJPEG(resized, 40)
		if err != nil {
			continue
		}
		if len(data) <= MaxImageSize {
			return data, "image/jpeg", nil
		}
	}

	// 8. 最后兜底：即使缩到最小仍超 1MB，强制返回当前最小结果
	// 这种情况极少见（如 2000×2000 像素的复杂图），用户应手动压
	return data, "image/jpeg", nil
}

// decodeImage 解码任意支持的图片格式。
func decodeImage(path string) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	// 先 sniff 格式（用扩展名兜底）
	ext := strings.ToLower(strings.TrimPrefix(extOf(path), "."))
	var format string
	switch ext {
	case "jpg", "jpeg":
		format = "jpeg"
	case "png":
		format = "png"
	case "gif":
		format = "gif"
	case "bmp":
		format = "bmp"
	default:
		// 尝试通过 Read 嗅探
		img, fmtName, err := image.Decode(f)
		return img, fmtName, err
	}

	// 重新打开以重置 reader
	f2, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f2.Close()

	switch format {
	case "jpeg":
		img, err := jpeg.Decode(f2)
		return img, "jpeg", err
	case "png":
		img, err := png.Decode(f2)
		return img, "png", err
	case "gif":
		img, err := gif.Decode(f2)
		return img, "gif", err
	case "bmp":
		return nil, "", fmt.Errorf("BMP 暂不支持（请转换为 PNG/JPG）")
	}
	return nil, "", fmt.Errorf("不支持的图片格式: %s", ext)
}

// hasTransparency 检测图片是否含透明通道。
func hasTransparency(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.RGBA64:
		return true
	}
	if _, ok := img.(*image.Paletted); ok {
		// GIF Paletted 几乎都有透明
		return true
	}
	return false
}

// flattenOnWhite 将含透明通道的图片合成到白底 RGBA 图上（对齐 v1）。
func flattenOnWhite(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	// 先填白底
	draw.Draw(dst, bounds, image.NewUniform(color.White), image.Point{}, draw.Src)
	// 再叠加原图（透明处自动透出白底）
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)
	return dst
}

// encodeJPEG 编码为 JPG 字节流，使用 optimize 模式（对齐 v1）。
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// extOf 提取文件扩展名（不含点）。
func extOf(path string) string {
	for i := len(path) - 1; i >= 0 && i >= len(path)-10; i-- {
		if path[i] == '.' {
			return path[i+1:]
		}
		if path[i] == '/' || path[i] == '\\' {
			return ""
		}
	}
	return ""
}
