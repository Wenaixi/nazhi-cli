// Package ocr 封装 ddddocr 验证码识别能力。
//
// 使用 //go:embed 将模型文件直接嵌入二进制，无需运行时下载。
// 首次调用 Recognize 时会自动将模型文件提取到临时目录。
package ocr

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yangbin1322/go-ddddocr/ddddocr"
)

// ─── 嵌入模型文件 ───

//go:embed models/common_old.onnx
var modelOnnx []byte

//go:embed models/charsets_old.json
var charsetJSON []byte

//go:embed models/onnxruntime.dll
var onnxRuntimeDLL []byte

// ─── OCR 服务 ───

// OCR 是验证码识别器，一旦初始化可重复使用。
type OCR struct {
	once    sync.Once
	initErr error
	ocr     *ddddocr.DdddOcr
	tempDir string
}

// New 创建 OCR 识别器（惰性初始化，首次调用时才提取模型文件）。
func New() *OCR {
	return &OCR{}
}

// Recognize 对图片字节进行验证码识别，返回识别出的文本。
// imageData 应为 JPEG 或 PNG 编码的字节。
func (o *OCR) Recognize(imageData []byte) (string, error) {
	o.once.Do(func() {
		o.tempDir, o.initErr = o.extractModels()
		if o.initErr != nil {
			return
		}

		// 设置 ONNX Runtime 路径为解压出的 DLL
		dllPath := filepath.Join(o.tempDir, "onnxruntime.dll")
		ddddocr.SetOnnxRuntimePath(dllPath)

		// 创建识别器，指定模型目录为解压目录
		opts := ddddocr.DefaultOptions()
		opts.ModelDir = o.tempDir
		ocr, err := ddddocr.New(opts)
		if err != nil {
			o.initErr = fmt.Errorf("创建 ddddocr 失败: %w", err)
			return
		}

		// 限制字符范围为 大写+小写+数字（验证码通常包含这些）
		ocr.SetRanges(ddddocr.RangeLowerUpperDigit)

		o.ocr = ocr
	})

	if o.initErr != nil {
		return "", fmt.Errorf("OCR 初始化失败: %w", o.initErr)
	}

	result, err := o.ocr.Classification(imageData)
	if err != nil {
		return "", fmt.Errorf("OCR 识别失败: %w", err)
	}

	return result, nil
}

// Close 释放 OCR 资源并清理临时文件。
func (o *OCR) Close() error {
	if o.ocr != nil {
		o.ocr.Close()
	}
	// 清理临时目录（忽略错误，让 OS 后续清理）
	if o.tempDir != "" {
		os.RemoveAll(o.tempDir)
	}
	return nil
}

// ─── 模型文件提取 ───

// extractModels 将内嵌的模型文件解压到系统临时目录。
// 这样做是因为 onnxruntime_go 需要从文件系统路径加载 DLL 和 ONNX 模型。
func (o *OCR) extractModels() (string, error) {
	// 在 OS 临时目录下创建 nazhi-cli 专属子目录
	dir, err := os.MkdirTemp("", "nazhi-cli-ocr-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 写入 onnxruntime.dll
	if err := writeFile(filepath.Join(dir, "onnxruntime.dll"), onnxRuntimeDLL); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("写入 onnxruntime.dll 失败: %w", err)
	}

	// 写入 common_old.onnx（模型文件，默认 OCR 模式使用）
	if err := writeFile(filepath.Join(dir, "common_old.onnx"), modelOnnx); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("写入 common_old.onnx 失败: %w", err)
	}

	// 写入 charsets_old.json
	if err := writeFile(filepath.Join(dir, "charsets_old.json"), charsetJSON); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("写入 charsets_old.json 失败: %w", err)
	}

	return dir, nil
}

// writeFile 写入文件，设置 0644 权限。
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
