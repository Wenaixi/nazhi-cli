package client

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	// 注册 WEBP 解码器（golang.org/x/image 是 disintegration/imaging 的间接依赖）
	_ "golang.org/x/image/webp"
)

// MaxImageSize 默认压缩目标上限（5MB）。
const MaxImageSize = 5 * 1024 * 1024

// MinImageDimension 缩放下限（像素），低于此值停止缩放。
const MinImageDimension = 10

// getQualitySteps 返回质量级联切片（每次返回新副本，保证不可变）。
// 从包级可变 var 改为函数返回，防止测试修改污染全局。
func getQualitySteps() []int { return []int{80, 60, 40} }

// getScaleFactors 返回缩放级联切片（每次返回新副本，保证不可变）。
// 从包级可变 var 改为函数返回，防止测试修改污染全局。
func getScaleFactors() []float64 { return []float64{0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7} }

// ErrImageTooLarge 压缩后仍超过 MaxImageSize。
var ErrImageTooLarge = errors.New("image exceeds maximum size after compression")

// ErrUnsupportedFormat 不支持的图片格式。
var ErrUnsupportedFormat = errors.New("unsupported image format")

// prepareImageForUpload 读取本地图片，预处理为符合平台要求的 JPG 字节流。
//
// 流程：
//  1. sniff 文件格式（magic bytes 优先，扩展名兜底）
//  2. 解码 + 透明合成 + 动画取首帧
//  3. 编码为 JPG（quality=92 起步）
//  4. 质量级联 → 缩放级联 → 输出
//
// 全部在内存中完成，不写盘、不修改原文件。
func (c *Client) prepareImageForUpload(path string) ([]byte, string, error) {
	// decodeImage 原来返回 format，自 GIF 特例删除后无消费者。
	// 无消费者。format 曾用于 `if format == "gif"` 分支，现已统一走
	// flattenOnWhite（hasTransparency 自动处理 Paletted 透明检测），
	// 故简化签名删除 format 返回值。
	img, err := decodeImage(path)
	if err != nil {
		return nil, "", err
	}

	// 透明合成：所有含透明通道的图片（NRGBA/RGBA/Paletted/GIF）都走 flattenOnWhite。
	//
	// 删除 `if format == "gif" && flattened` 特例分支。
	// 原特例做两件事——imaging.Clone(img) 丢弃透明 + flattened=false 跳过
	// flattenOnWhite——结果 GIF 透明区域经 jpeg.Encode 被解析为黑色（黑底）。
	// 失败场景：用户上传带透明 GIF → 服务端收到黑底 JPG → 视觉错误。
	//
	// hasTransparency 对 *image.Paletted 始终返回 true（GIF 解码几乎都是
	// Paletted），删除特例后 GIF 透明索引会经 flattenOnWhite 合成到白底，
	// 与 PNG/NRGBA 透明处理契约一致。
	flattened := hasTransparency(img)
	if flattened {
		img = flattenOnWhite(img)
	}

	// 尝试用 92 起步
	data, err := encodeJPEG(img, 92)
	if err != nil {
		return nil, "", fmt.Errorf("JPG 编码失败: %w", err)
	}

	// 已满足
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}

	// 质量级联
	for _, q := range getQualitySteps() {
		data, err = encodeJPEG(img, q)
		if err != nil {
			return nil, "", fmt.Errorf("质量 %d 编码失败: %w", q, err)
		}
		if len(data) <= MaxImageSize {
			return data, "image/jpeg", nil
		}
	}

	// 缩放级联（保持质量 40，每步基于上一步结果累乘 ×scale）
	current := img
	for _, scale := range getScaleFactors() {
		b := current.Bounds()
		w := int(float64(b.Dx()) * scale)
		h := int(float64(b.Dy()) * scale)
		if w < MinImageDimension || h < MinImageDimension {
			break
		}
		resized := imaging.Resize(current, w, h, imaging.Lanczos)
		data, err = encodeJPEG(resized, 40)
		if err != nil {
			// break 而非 continue。
			//
			// 原代码 `continue` 会跳过下面的 `current = resized`，下一轮用
			// 未更新的 current 计算 w/h → 同一尺寸重复 encodeJPEG 必然同样失败
			// （jpeg encoder 内部错误是确定性的，重试无意义）→ 浪费 1-7 轮
			// CPU 后才在 MinImageDimension 边界 break 返回 ErrImageTooLarge。
			//
			// 修复：break + logDebug，让失败原因可观测，立即进入兜底逻辑。
			c.logDebug("缩放级联 encodeJPEG 失败，跳出循环：scale=%v err=%v", scale, err)
			break
		}
		if len(data) <= MaxImageSize {
			return data, "image/jpeg", nil
		}
		// 关键：下一轮基于当前 resized 而非原图（累乘语义）
		current = resized
	}

	// 兜底：返回当前最小结果
	if data == nil {
		return nil, "", ErrImageTooLarge
	}
	// 兜底前检查大小，若仍超限返回错误（避免首次 break 就兜底的边界 bug）
	if len(data) > MaxImageSize {
		return nil, "", ErrImageTooLarge
	}
	return data, "image/jpeg", nil
}

// decodeImage sniff 文件 magic bytes 解码任意格式。
// 优先用 magic bytes 检测，避免依赖扩展名（用户可能给 .dat 文件）。
//
// 删除 format 返回值，无消费者。
func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	// magic bytes sniff
	var head [12]byte
	n, err := io.ReadFull(f, head[:])
	if n == 0 {
		return nil, errors.New("file is empty")
	}
	if err != nil {
		return nil, fmt.Errorf("读取图片文件头失败: %w", err)
	}

	format := sniffFormat(head[:n])
	if format == "" {
		// 用扩展名兜底
		format = strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	}

	// 重置 reader
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("读取图片失败: %w", err)
	}

	switch format {
	case "jpeg", "jpg":
		img, err := jpeg.Decode(f)
		return img, err
	case "png":
		img, err := png.Decode(f)
		return img, err
	case "gif":
		img, err := gif.Decode(f)
		return img, err
	case "webp":
		img, err := decodeWebP(f)
		return img, err
	case "bmp":
		// stdlib 无 BMP 解码，提示用户转换
		return nil, fmt.Errorf("%w: BMP（请先用图片工具转为 PNG/JPG）", ErrUnsupportedFormat)
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
}

// sniffFormat 通过文件头 magic bytes 识别格式。
func sniffFormat(head []byte) string {
	if len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF {
		return "jpeg"
	}
	if len(head) >= 8 && head[0] == 0x89 && head[1] == 'P' && head[2] == 'N' && head[3] == 'G' {
		return "png"
	}
	// 用 bytes.Equal 避免 string(head[:6]) 堆分配（字面量已在 .rodata 分配好，
	// 但 []byte→string 转换在 go 编译时无法逃逸分析，实际触发堆分配）。
	if len(head) >= 6 && (bytes.Equal(head[:6], []byte("GIF87a")) || bytes.Equal(head[:6], []byte("GIF89a"))) {
		return "gif"
	}
	// WEBP: "RIFF" + 4 bytes + "WEBP"
	if len(head) >= 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")) {
		return "webp"
	}
	return ""
}

// decodeWebP 包装 webp.Decode 并提供友好错误。
func decodeWebP(r io.Reader) (image.Image, error) {
	img, err := imaging.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("WEBP 解码失败: %w", err)
	}
	return img, nil
}

// hasTransparency 检测图片是否含透明通道。
//
// 将 *image.Paletted 独立 if 合并到 type switch 中，
// 消除独立的 if 语句，使透明检测逻辑更紧凑。
func hasTransparency(img image.Image) bool {
	switch img.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.RGBA64:
		return true
	case *image.Paletted:
		// GIF Paletted 几乎都有透明
		return true
	}
	return false
}

// flattenOnWhite 将含透明通道的图片合成到白底 RGBA 图上。
func flattenOnWhite(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)
	return dst
}

// jpegBufPool 复用 bytes.Buffer 给 encodeJPEG，避免每次上传 5MB 图片时
// cascade 重编码 2-11 次的 buffer 重复分配/GC 压力。
// 注：bytes.Buffer.Get 出来必须 Reset；返回的 []byte 必须 copy（pool Put 后
// 内部 slice 会被其他 goroutine 复用覆盖）。
var jpegBufPool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

// encodeJPEG 编码为 JPG 字节流。
// 使用 sync.Pool 复用 buffer 减少 GC 压力，cascade 重编码场景下
// 5MB 图片多次 encode 共享同一个 buffer 实例。
func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	buf, ok := jpegBufPool.Get().(*bytes.Buffer)
	if !ok {
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	defer func() {
		// 释放前清空，避免 buffer 持有对 img 像素的引用导致 GC 无法回收
		buf.Reset()
		jpegBufPool.Put(buf)
	}()
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	// 关键：必须 copy 出来再返回——pool Put 后 buffer 内部 slice 会被
	// 其他 goroutine 复用，buf.Bytes() 返回的引用会立刻失效
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}
