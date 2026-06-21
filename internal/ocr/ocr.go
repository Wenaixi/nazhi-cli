// Package ocr 封装 ddddocr 验证码识别能力。
//
// 使用 //go:embed 将模型文件直接嵌入二进制，无需运行时下载。
// 首次调用 Recognize 时会自动将模型文件提取到临时目录。
//
// 跨平台支持：原生库（onnxruntime）按 (GOOS, GOARCH) 用 build tag 隔离
// 嵌入到不同的源文件（onnx_win_amd64.go 等），每个平台只携带自己那份。
package ocr

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/yangbin1322/go-ddddocr/ddddocr"
)

// ─── 跨平台模型文件 ───

//go:embed models/common_old.onnx
var modelOnnx []byte

//go:embed models/charsets_old.json
var charsetJSON []byte

// OnnxRuntimeDLL 由 build-tag 隔离的 4 个文件之一提供（见 onnx_*.go）。
// 这里不能用 //go:embed 单一文件，因为各平台的原生库二进制完全不同。

// ─── OCR 服务 ───

// OCR 是验证码识别器，一旦初始化可重复使用。
// 多 Client 推荐使用 GetDefault() 共享进程级单例，避免重复解压模型。
type OCR struct {
	once    sync.Once
	initErr error
	ocr     *ddddocr.DdddOcr
	tempDir string
	mu      sync.Mutex // 保护 Classification 调用，支持单例并发安全
}

// New 创建独立的 OCR 识别器（惰性初始化，首次调用时才提取模型文件）。
// 业务代码一般用 GetDefault() 共享单例；测试可以用 New() 创建隔离实例。
func New() *OCR {
	return &OCR{}
}

// GetDefault 返回进程级 OCR 单例。所有 Client 共享同一引擎，
// 模型只解压一次，多个 Client 不再产生多个临时目录。
func GetDefault() *OCR {
	defaultOnce.Do(func() {
		defaultOCR = &OCR{}
	})
	return defaultOCR
}

var (
	defaultOCR  *OCR
	defaultOnce sync.Once
)

// ─── 平台文件名 ───

// platformLibName 根据 runtime.GOOS 返回解压到磁盘时的原生库文件名。
// C 运行时需要按平台命名规范来 LoadLibrary / dlopen。
func platformLibName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "linux":
		return "libonnxruntime.so"
	default:
		// 不支持的平台调用时会得到无扩展名文件，ddddocr.SetOnnxRuntimePath 会失败
		return "onnxruntime"
	}
}

// ─── 识别 API ───

// Recognize 对图片字节进行验证码识别，返回识别出的文本。
// imageData 应为 JPEG 或 PNG 编码的字节。
func (o *OCR) Recognize(imageData []byte) (string, error) {
	o.once.Do(func() {
		o.tempDir, o.initErr = o.extractModels()
		if o.initErr != nil {
			return
		}

		// 设置 ONNX Runtime 路径为解压出的原生库
		libPath := filepath.Join(o.tempDir, platformLibName())
		ddddocr.SetOnnxRuntimePath(libPath)

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

	// 单例场景下保护 Classification 调用，并发请求时串行执行识别
	o.mu.Lock()
	result, err := o.ocr.Classification(imageData)
	o.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("OCR 识别失败: %w", err)
	}

	return result, nil
}

// Close 释放 OCR 资源并清理临时文件。
// 进程退出时操作系统会回收临时目录，这里只主动 Close 引擎。
func (o *OCR) Close() error {
	if o.ocr != nil {
		o.ocr.Close()
	}
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

	// 写入原生库（按当前平台命名）
	libName := platformLibName()
	if err := writeFile(filepath.Join(dir, libName), OnnxRuntimeDLL); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("写入 %s 失败: %w", libName, err)
	}

	// 写入 ONNX 模型
	if err := writeFile(filepath.Join(dir, "common_old.onnx"), modelOnnx); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("写入 common_old.onnx 失败: %w", err)
	}

	// 写入字符集
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
