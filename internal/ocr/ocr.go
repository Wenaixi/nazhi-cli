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
	"errors"
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

// OnnxRuntimeDLL 由 build-tag 隔离的 5 个文件之一提供（见 onnx_*.go）。
// 这里不能用 //go:embed 单一文件，因为各平台的原生库二进制完全不同。

// ─── OCR 服务 ───

// OCR 是验证码识别器，一旦初始化可重复使用。
// 多 Client 推荐使用 GetDefault() 共享进程级单例，避免重复解压模型。
type OCR struct {
	initMu      sync.Mutex // 保护初始化路径和 closed 翻转
	initialized bool      // true = 初始化已完成（成功或失败由 initErr 决定）
	closed      bool      // true = Close() 已调用，禁止后续识别
	initErr     error
	ocr         *ddddocr.DdddOcr
	tempDir     string
	mu          sync.Mutex // 保护 Classification 调用，支持单例并发安全
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

// Pool 是多个 OCR 实例的池，允许并发识别（默认 1 实例，兼容单例行为）。
//
// ONNX Runtime session 不是线程安全的（一个 session 同一时刻只能一个线程调用），
// 所以单实例下并发请求会被 sync.Mutex 串行化，N 并发 Login 的 wall time = N × 单次延迟。
//
// 启用并发：NewPool(n) 预热 n 个独立 session 实例，允许 n 路真并发。
// 内存代价：每个实例约 50MB（ONNX 模型 + 原生库解压到独立 tempDir），n=4 ≈ 200MB。
// 业务场景：批量调用 Login() 时才需要调高；单 Login 调一次用 1 实例足够。
type Pool struct {
	pool    sync.Pool
	initsMu sync.Mutex
	inits   map[*OCR]struct{} // 跟踪所有完成过惰性初始化的实例，供 Close() 释放
}

// NewPool 创建 OCR 实例池。preload=0 或 1 表示懒加载单实例（默认行为）。
// preload>1 表示预分配 n 个 OCR 结构体（ONNX session 仍惰性初始化，首次 Recognize 时触发）。
// 注意：sync.Pool 在 GC 后可能回收对象，到时惰性初始化兜底。
func NewPool(preload int) *Pool {
	p := &Pool{
		pool:  sync.Pool{New: func() any { return &OCR{} }},
		inits: make(map[*OCR]struct{}),
	}
	for i := 0; i < preload; i++ {
		// 预热：先 Get 触发 New，初始化 session，再 Put 回 pool
		o, ok := p.pool.Get().(*OCR)
		if !ok {
			o = &OCR{}
		}
		p.trackInit(o)
		p.pool.Put(o)
	}
	return p
}

// trackInit 记录首次完成惰性初始化的 OCR 实例。
// 重复 Put 同一对象 (Get/Put 对) 不会重复入 map。
func (p *Pool) trackInit(o *OCR) {
	p.initsMu.Lock()
	p.inits[o] = struct{}{}
	p.initsMu.Unlock()
}

// Recognize 从池中取一个 OCR 实例识别图片，用完归还。
// 不同实例并发安全（每个实例内部有独立 mu 保护 Classification）。
func (p *Pool) Recognize(imageData []byte) (string, error) {
	o, ok := p.pool.Get().(*OCR)
	if !ok {
		o = &OCR{}
	}
	p.trackInit(o)
	defer p.pool.Put(o)
	return o.Recognize(imageData)
}

// Close 释放池中所有已完成惰性初始化的 OCR 实例 (ONNX session + 临时目录)。
//
// 注意：sync.Pool 持有的是结构体, 真正需要 Close 的是初始化过 session 的实例。
// 池内只跟踪首次完成初始化 (Recognize 后) 的实例, 避免漏释放或重复释放。
//
// 多实例池 (NewPool(N>1)) 下, 每个实例对应独立 tempDir, Close 会释放全部。
func (p *Pool) Close() error {
	p.initsMu.Lock()
	inits := p.inits
	p.inits = make(map[*OCR]struct{})
	p.initsMu.Unlock()

	var errs []error
	for o := range inits {
		if err := o.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
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
	// 用 initMu 保护整个初始化路径，确保多 goroutine 下也只解压一次。
	// 关键设计：Close() 会把 closed=true、initErr=nil、initialized=false、
	// o.ocr=nil，之后调 Recognize 走「重新初始化」分支（除非已 close）。
	o.initMu.Lock()
	if o.closed {
		o.initMu.Unlock()
		return "", errors.New("OCR 已关闭")
	}
	if !o.initialized {
		o.tempDir, o.initErr = o.extractModels()
		if o.initErr != nil {
			o.initialized = true
			o.initMu.Unlock()
			return "", fmt.Errorf("OCR 初始化失败: %w", o.initErr)
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
			o.initialized = true
			o.initMu.Unlock()
			return "", o.initErr
		}

		// 限制字符范围为 大写+小写+数字（验证码通常包含这些）
		ocr.SetRanges(ddddocr.RangeLowerUpperDigit)

		o.ocr = ocr
		o.initialized = true
	}
	initErr := o.initErr
	ocr := o.ocr
	o.initMu.Unlock()

	if initErr != nil {
		return "", fmt.Errorf("OCR 初始化失败: %w", initErr)
	}
	// 防御：initialized=true 但 ocr=nil 是不一致状态（Close 后、初始化中途异常等）
	if ocr == nil {
		return "", errors.New("OCR 不可用：识别器为 nil")
	}

	// 单例场景下保护 Classification 调用，并发请求时串行执行识别
	o.mu.Lock()
	result, err := ocr.Classification(imageData)
	o.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("OCR 识别失败: %w", err)
	}

	return result, nil
}

// Close 释放 OCR 资源并清理临时文件。
// 返回任何清理过程中遇到的错误（Windows AV 持锁、Linux 权限拒绝等场景），
// 让调用方知情，避免临时目录永久泄漏到 %TEMP%。
//
// Close 后再次调用 Recognize 会返回 "OCR 已关闭" 错误，而不是触发 nil panic。
func (o *OCR) Close() error {
	o.initMu.Lock()
	o.closed = true
	o.initialized = false
	o.initErr = nil
	ocr := o.ocr
	o.ocr = nil
	tempDir := o.tempDir
	o.tempDir = ""
	o.initMu.Unlock()

	var errs []error
	if ocr != nil {
		if err := ocr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭 ddddocr 引擎: %w", err))
		}
	}
	if tempDir != "" {
		if err := os.RemoveAll(tempDir); err != nil {
			errs = append(errs, fmt.Errorf("清理临时目录 %s: %w", tempDir, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
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
