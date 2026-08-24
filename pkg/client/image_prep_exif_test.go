package client

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"
)

// buildJPEGWithOrientation 生成一张带 EXIF Orientation 标记的 JPEG。
// exifPayload 是从 TIFF 头开始的字节（不含 "Exif\x00\x00" 前缀）。
func buildJPEGWithOrientation(t *testing.T, w, h int, exifPayload []byte) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{200, 200, 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("jpeg 编码失败: %v", err)
	}

	// 构造 APP1(EXIF) 段并插入 SOI 之后
	exifPrefix := []byte{'E', 'x', 'i', 'f', 0, 0}
	// exifPayload 自带完整 TIFF 头（II 字节序 + 42 + IFD offset），不再额外填充
	body := append(exifPrefix, exifPayload...)
	seg := append([]byte{0xFF, 0xE1}, byte((len(body)+2)>>8), byte((len(body)+2)%256))
	seg = append(seg, body...)

	out := buf.Bytes()
	// SOI(2) + APP0(JFIF, 通常18字节头) —— 找第二个 0xFF marker 插入点：直接插在 SOI 后
	var outBuf bytes.Buffer
	outBuf.Write(out[:2]) // SOI
	outBuf.Write(seg)
	outBuf.Write(out[2:])
	return outBuf.Bytes()
}

// orientation6TIFF 返回一个最小 TIFF 负载：II 字节序 + IFD0 仅含 Orientation=6 (tag 0x0112)。
func orientation6TIFF() []byte {
	// II 字节序(0x4949) + 42 + IFD0 offset=8
	head := []byte{'I', 'I', 42, 0, 8, 0, 0, 0}
	// IFD: 1 个条目 + 下一个 IFD offset(4)
	entryCount := []byte{1, 0}
	// 条目: tag=0x0112(Orientation) type=3(SHORT) count=1 value=6
	entry := []byte{
		0x12, 0x01, // tag little-endian
		0x03, 0x00, // type SHORT
		0x01, 0x00, 0x00, 0x00, // count
		0x06, 0x00, 0x00, 0x00, // value=6（SHORT 内联在低 16 位）
	}
	nextIFD := []byte{0, 0, 0, 0}
	return append(append(head, entryCount...), append(entry, nextIFD...)...)
}

// TestPrepareImage_AppliesEXIFOrientation 验证带 Orientation=6 的 JPEG 经预处理后
// 已按 EXIF 摆正（存储 100x50 竖内容应变为 50x100 输出）。
// 对齐前端行为：canvas drawImage 在现代浏览器默认按 EXIF 摆正后再编码上传。
func TestPrepareImage_AppliesEXIFOrientation(t *testing.T) {
	c := internalNewTestClient()

	raw := buildJPEGWithOrientation(t, 100, 50, orientation6TIFF())
	tmpfile := t.TempDir() + "/exif-orient6.jpg"
	if err := os.WriteFile(tmpfile, raw, 0o644); err != nil {
		t.Fatalf("写入夹具失败: %v", err)
	}

	data, _, err := c.prepareImageForUpload(tmpfile)
	if err != nil {
		t.Fatalf("prepareImageForUpload 失败: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("输出不是合法 JPEG: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() != 50 || b.Dy() != 100 {
		t.Fatalf("EXIF Orientation=6 未被应用：期望输出 50x100（已摆正），实际 %dx%d", b.Dx(), b.Dy())
	}
}