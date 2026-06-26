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

// ─── 进程级全局同步 ───

// B6 修复：SetOnnxRuntimePath 是进程级全局函数，Pool 多实例并发初始化时
// 需要全局互斥锁保护 SetOnnxRuntimePath + ddddocr.New 两步的组合原子性。
//
// 设计：
//   - onceSetPath 确保 SetOnnxRuntimePath 在整个进程生命周期只调用一次
//   - initMuGlobal 保护 New + SetOnnxRuntimePath 组合原子性，
//     防止 G1 调 SetOnnxRuntimePath(pathA) → G2 调 SetOnnxRuntimePath(pathB)
//     → G1 的 ddddocr.New(opts) 读到 pathB
var (
	onceSetPath  sync.Once
	initMuGlobal sync.Mutex
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
// 多 Client 推荐共享同一个 Pool 实例（见 client.New 的 WithOCRConcurrency），
// 避免重复解压模型。
type OCR struct {
	initMu      sync.Mutex // 保护初始化路径和 closed 翻转
	initialized bool       // true = 初始化已完成（成功或失败由 initErr 决定）
	closed      bool       // true = Close() 已调用，禁止后续识别
	initErr     error
	ocr         *ddddocr.DdddOcr
	tempDir     string
	mu          sync.Mutex // 保护 Classification 调用，支持单例并发安全
}

// New 创建独立的 OCR 识别器（惰性初始化，首次调用时才提取模型文件）。
// 业务代码一般用 Pool 实例共享单例引擎；测试可以用 New() 创建隔离实例。
func New() *OCR {
	return &OCR{}
}

// Pool 是多个 OCR 实例的池，允许并发识别（默认 1 实例，兼容单例行为）。
//
// ONNX Runtime session 不是线程安全的（一个 session 同一时刻只能一个线程调用），
// 所以单实例下并发请求会被 sync.Mutex 串行化，N 并发 Login 的 wall time = N x 单次延迟。
//
// 启用并发：NewPool(n) 预热 n 个独立 session 实例，允许 n 路真并发。
// 内存代价：每个实例约 50MB（ONNX 模型 + 原生库解压到独立 tempDir），n=4 ≈ 200MB。
// 业务场景：批量调用 Login() 时才需要调高；单 Login 调一次用 1 实例足够。
//
// inits 字段用 sync.Map 存储已注册实例，原因：
//   - O7 修复：99 次串行 Recognize = 99 次 trackInit(同一 *OCR)
//   - sync.Map.LoadOrStore 在 key 已存在时是 lock-free 路径，
//     避免 mutex.Lock + map 写入的固定开销
//   - sync.Map 读写并发安全，无需额外的 initsMu 保护
//   - Close 路径用 Range 迭代，配合 closeOnce 仍保证只跑一次 Close 工作
//
// F2 + F22 合并修复：用 closeMu 保护整个「read closed + Get + trackInit」
//
//	原子临界区 + Close 的「Range(inits) + 翻 closed」原子临界区。
//	两个临界区互斥（同一把 mutex），保证并发 Recognize 不会被 close window 切断。
//	为什么不用 atomic.Bool.Load：atomic.Load + Get + trackInit 在 Go 内存模型下
//	不是原子的（Load 之后到后续语句之间 goroutine 可被调度走，Close 在此期间
//	完成 Range + 翻 closed，但 goroutine 已被 Load(false) 误导，仍会 trackInit
//	到 inits map 内 -> 泄漏）。所以需要 mutex 临界区。
//	简化：closeOnce 仍然存在（保证 Close 关键路径只跑一次 + 错误聚合），
//	closeMu 在 Close 内现在只保护临界区入口（进/出 closeMu），与 closeOnce 配合。
type Pool struct {
	pool      sync.Pool
	inits     sync.Map // key=*OCR, value=struct{}：跟踪所有完成过惰性初始化的实例
	closeMu   sync.Mutex
	closeOnce sync.Once
	closed    bool
}

// NewPool 创建 OCR 实例池。preload=0 或 1 表示懒加载单实例（默认行为）。
// preload>1 表示预分配 n 个 OCR 结构体（ONNX session 仍惰性初始化，首次 Recognize 时触发）。
// 注意：sync.Pool 在 GC 后可能回收对象，到时惰性初始化兜底。
func NewPool(preload int) *Pool {
	p := &Pool{
		pool: sync.Pool{New: func() any { return &OCR{} }},
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
// 用 sync.Map.LoadOrStore 实现「key 已存在时 lock-free 跳过」——
//
//	O7 优化：99 次串行 Recognize(同一 *OCR) 只触发 1 次实际 Store，
//	其余 98 次走 LoadOrStore 的「已存在」快速路径（无 mutex.Lock 开销）。
func (p *Pool) trackInit(o *OCR) {
	p.inits.LoadOrStore(o, struct{}{})
}

// Recognize 从池中取一个 OCR 实例识别图片，用完归还。
// 不同实例并发安全（每个实例内部有独立 mu 保护 Classification）。
//
// Pool.Close 后调用 Recognize 返回"OCR 池已关闭"错误，防止新创建的
// OCR 实例泄漏 tempDir（Pool.Close 的 inits.Range 排空后再创建的实例
// 不会被 Close 路径清理）。
//
// F2 关键修复：close 检查 + pool.Get + trackInit 必须在同一 mutex 临界区内。
//
//	否则并发 Recognize 可穿过 Close 的 Range 完成窗口，向 inits map
//	注册新实例但 Close 已不会再次访问 -> tempDir 永久泄漏。
//	具体场景：
//	  T0 Close 进入 closeOnce -> 拿 closeMu -> Range(inits) -> 翻 closed -> 放 closeMu
//	  T1 Recognize 拿 closeMu（在 T0 之后）-> 看到 closed=true -> 直接返回错误
//	  T1' Recognize 拿 closeMu（在 T0 之前）-> 看到 closed=false -> Get + trackInit
//	       -> 放 closeMu -> Close 路径（拿 closeMu）-> Range 包含 T1' 注册的实例
//	保证：T1 要么被 Close 之前完整处理（被 Close 清理），要么被 Close 之后拒绝。
func (p *Pool) Recognize(imageData []byte) (string, error) {
	// 临界区：close 检查 + Get + trackInit 原子完成
	// （不能调 o.Recognize，避免 ONNX 识别阻塞在临界区内）
	var o *OCR
	p.closeMu.Lock()
	if !p.closed {
		o, _ = p.pool.Get().(*OCR)
		if o == nil {
			o = &OCR{}
		}
		p.trackInit(o)
	}
	p.closeMu.Unlock()
	if o == nil {
		return "", errors.New("OCR 池已关闭")
	}

	defer p.pool.Put(o)
	return o.Recognize(imageData)
}

// Close 释放池中所有已完成惰性初始化的 OCR 实例 (ONNX session + 临时目录)。
//
// 注意：sync.Pool 持有的是结构体, 真正需要 Close 的是初始化过 session 的实例。
// 池内只跟踪首次完成初始化 (Recognize 后) 的实例, 避免漏释放或重复释放。
//
// 多实例池 (NewPool(N>1)) 下, 每个实例对应独立 tempDir, Close 会释放全部。
//
// 并发安全（F1 修复）：用 sync.Once 保证"排空 map + 迭代 Close 实例"这一段
// 关键路径只跑一次。即使多个 goroutine 同时调 Close，第一次调用的协程
// 负责全部释放工作，后续调用立即返回 nil，避免同一实例被 Close 两次。
//
// F2 修复：closeMu 保护「Range(inits) + 翻 closed」原子临界区，与 Recognize
//
//	路径的「读 closed + Get + trackInit」临界区互斥。任何并发 Recognize 要么：
//	  1) 在 Close 临界区之前完成 trackInit -> 被 Range 清理
//	  2) 在 Close 临界区之后拿 closeMu -> 看到 closed=true -> 直接返回错误
//	不会有"漏网"的 trackInit 留下幽灵实例。
//
// O7 优化：Pool.inits 是 sync.Map（无独立 initsMu），Close 路径用 Range
// 原子快照迭代——sync.Map.Range 在迭代期间对后续 Load/Store 安全，
// 配合 sync.Once 保证排空分支只跑一次。
func (p *Pool) Close() error {
	var firstErr error
	p.closeOnce.Do(func() {
		p.closeMu.Lock()
		var errs []error
		p.inits.Range(func(key, _ any) bool {
			o, ok := key.(*OCR)
			if !ok {
				return true
			}
			// 排空：Range 期间对已访问 key 做 Delete，下次 Range 不再返回
			p.inits.Delete(o)
			if err := o.Close(); err != nil {
				errs = append(errs, err)
			}
			return true
		})
		p.closed = true
		p.closeMu.Unlock()

		if len(errs) > 0 {
			firstErr = errors.Join(errs...)
		}
	})
	return firstErr
}

// ─── 平台文件名 ───

// platformLibName 根据 runtime.GOOS 返回解压到磁盘时的原生库文件名。
// C 运行时需要按平台命名规范来 LoadLibrary / dlopen。
func platformLibName() string {
	return platformLibNameFor(runtime.GOOS)
}

// platformLibNameFor 是 platformLibName 的参数化版本，接受任意 GOOS 字符串。
// B5 修复：补充 darwin 分支返回 .dylib 扩展名。
func platformLibNameFor(goos string) string {
	switch goos {
	case "windows":
		return "onnxruntime.dll"
	case "linux":
		return "libonnxruntime.so"
	case "darwin":
		return "libonnxruntime.dylib"
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

		// B6 修复：SetOnnxRuntimePath + ddddocr.New 用 initMuGlobal 保护，
		// 确保多实例并发初始化时 SetOnnxRuntimePath 和 New 不被交叉覆盖。
		// onceSetPath 确保 SetOnnxRuntimePath 在整个进程中只调用一次。
		initMuGlobal.Lock()
		libPath := filepath.Join(o.tempDir, platformLibName())
		onceSetPath.Do(func() {
			ddddocr.SetOnnxRuntimePath(libPath)
		})

		// 创建识别器，指定模型目录为解压目录
		opts := ddddocr.DefaultOptions()
		opts.ModelDir = o.tempDir
		ocr, err := ddddocr.New(opts)
		initMuGlobal.Unlock()
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
	if err := writeModelFile(dir, libName, OnnxRuntimeDLL); err != nil {
		return "", err
	}

	// 写入 ONNX 模型
	if err := writeModelFile(dir, "common_old.onnx", modelOnnx); err != nil {
		return "", err
	}

	// 写入字符集
	if err := writeModelFile(dir, "charsets_old.json", charsetJSON); err != nil {
		return "", err
	}

	return dir, nil
}

// writeModelFile 写入模型文件并设置 0644 权限。
// 写入失败时自动清理临时目录。
// C8 修复：提取为 helper，消除三次 writeFile + os.RemoveAll + fmt.Errorf 的重复模式。
func writeModelFile(dir, name string, data []byte) error {
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("写入 %s 失败: %w", name, err)
	}
	return nil
}
