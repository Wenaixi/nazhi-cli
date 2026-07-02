package client

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"sync"

	"github.com/disintegration/imaging"
	// WEBP 解码器通过 image.RegisterFormat 注册，image.Decode 自动派发
	_ "golang.org/x/image/webp"
)

// MaxImageSize 默认压缩目标上限（5MB）。
const MaxImageSize = 5 * 1024 * 1024

// MinImageDimension 缩放下限（像素），低于此值停止缩放。
const MinImageDimension = 10

// qualityAfterOptimization 图片质量预设，经 F8.1 优化后的取值。
// 80% 的场景 quality=80 足够压到 ≤5MB，不够的走缩放更高效。
const qualityAfterOptimization = 80

// ErrImageTooLarge 压缩后仍超过 MaxImageSize。
var ErrImageTooLarge = errors.New("image exceeds maximum size after compression")

// ErrUnsupportedFormat 不支持的图片格式。
var ErrUnsupportedFormat = errors.New("unsupported image format")

// prepareImageForUpload 读取本地图片，预处理为符合平台要求的 JPG 字节流。
//
// 流程：
//  1. 魔术字节 sniff 文件格式，使用 image.Decode 自动派发
//  2. 解码 + 透明合成 + 动画取首帧
//  3. 编码为 JPG（quality=92 起步）
//  4. 质量级联 → 缩放 → 输出
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

	// F8.1 优化：如果 data 远超上限（>2×MaxImageSize），跳过质量级联（省三次 encode），
	// 直接进缩放级联。quality=80 对超大图片通常不够降到 ≤5MB，缩放最少省 50% 体积。
	q := qualityAfterOptimization
	if len(data) > 2*MaxImageSize {
		goto scaleCascade
	}

	// 质量级联（只跑一次 quality=80）
	data, err = encodeJPEG(img, q)
	if err != nil {
		return nil, "", fmt.Errorf("质量 %d 编码失败: %w", q, err)
	}
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}

	// F8.1 优化：添加 scaleCascade 标签，质量级联跳过时直接跳入
scaleCascade:
	// 单次缩放取代 7 轮累乘：0.7^7 ≈ 0.082，避免 4K 图 ~200MB 临时内存。
	b := img.Bounds()
	finalW := int(float64(b.Dx()) * 0.082)
	finalH := int(float64(b.Dy()) * 0.082)
	current := img
	if finalW >= MinImageDimension && finalH >= MinImageDimension {
		current = imaging.Resize(img, finalW, finalH, imaging.Lanczos)
	}

	// 统一编码为 JPG（quality=40），只 encode 一次
	data, err = encodeJPEG(current, 40)
	if err != nil {
		c.logDebug("缩放级联最终 encodeJPEG 失败：err=%v", err)
		return nil, "", fmt.Errorf("缩放级联编码失败: %w", err)
	}
	if len(data) <= MaxImageSize {
		return data, "image/jpeg", nil
	}
	// 兜底：缩小到极限仍超限
	return nil, "", ErrImageTooLarge
}

// decodeImage 使用 stdlib image.Decode 解码，自动通过魔术字节派发到
// 已注册的格式（jpeg/png/gif/webp）。
//
// 不再需要手动 switch — image.Decode 通过各包的 init() 注册的魔术字节
// 自动匹配。BMP 在解码失败后检测魔术字节单独报错。
func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		// 检测是否是 BMP（stdlib 不支持），给出友好提示
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr == nil {
			var magic [2]byte
			if _, readErr := io.ReadFull(f, magic[:]); readErr == nil && magic[0] == 'B' && magic[1] == 'M' {
				return nil, fmt.Errorf("%w: BMP（请先用图片工具转为 PNG/JPG）", ErrUnsupportedFormat)
			}
		}
		return nil, fmt.Errorf("图片解码失败: %w", err)
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
