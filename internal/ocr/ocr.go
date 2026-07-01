// Package ocr 封装 ddddocr 验证码识别能力。
//
// 使用 //go:embed 将模型文件直接嵌入二进制，无需运行时下载。
// 首次调用 Recognize 时会自动将模型文件提取到临时目录。
//
// 跨平台支持：原生库（onnxruntime）按 (GOOS, GOARCH) 用 build tag 隔离
// 嵌入到不同的源文件（onnx_win_amd64.go 等），每个平台只携带自己那份。
package ocr

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/yangbin1322/go-ddddocr/ddddocr"
)

// ─── 临时目录清理（含 Windows DLL 占用降级）───

// removeDirFn 是 cleanupTempDir 注入的目录删除函数，便于测试 mock。
// 默认是 os.RemoveAll。测试可通过赋值换成可控返回的 mock。
var removeDirFn = os.RemoveAll

// cleanupTempDir 删除给定的临时目录，并对 OS 级「文件被占用」类错误降级。
//
// 为什么需要降级：在 Windows 上运行时，OCR 引擎初始化时会通过 CGO
// LoadLibrary 加载 onnxruntime.dll；Windows 在进程退出前不会释放该 DLL
// 的 mmap 文件句柄。当 Close() 在进程退出前调用 os.RemoveAll 删临时目录时，
// 删到 onnxruntime.dll 时会被 OS 拒绝：
//
//	ERROR_ACCESS_DENIED       (errno 5)
//	ERROR_SHARING_VIOLATION   (errno 32)
//
// 这种 OS 级占用错误会在进程退出时由 OS 自然清理（句柄释放 → 文件可删），
// 没必要把非「业务可控」的 stderr 污染当作 Close 失败上报。
//
// 「不静默吞错」铁律保留：仅 Windows 下 DLL/原生库占用导致的两类 errno 才降级，
// 其他错误（Linux EPERM、磁盘满、只读卷、路径不存在等）照常返回。
//
// 跨平台：由 isPlatformLibBusy 的 GOOS 守卫保证降级只在 Windows 生效，
// Linux/macOS 上数值相同的 errno（EIO=5、EPIPE=32）不会被错认为「DLL 占用」。
//
// 注意：这里不构造 fs.ErrPermission 短路，因为 Linux 上权限拒绝（perm denied）
// 是真实的环境问题，需要让调用方知情。
func cleanupTempDir(dir string) error {
	if dir == "" {
		return nil
	}
	err := removeDirFn(dir)
	if err == nil {
		return nil
	}
	if isPlatformLibBusy(err) {
		// ponytail: OS 级 DLL 占用错误是进程级状态副作用，进程退出时自愈，
		// 上报反而污染 stderr。限定 errno 不放宽，避免吞真错误。
		return nil
	}
	return fmt.Errorf("清理临时目录 %s: %w", dir, err)
}

// isPlatformLibBusy 判定 OS 级「文件被占用」类错误。当前覆盖：
//
//	Windows syscall.Errno == ERROR_ACCESS_DENIED       (5)
//	Windows syscall.Errno == ERROR_SHARING_VIOLATION   (32)
//
// 用 errors.As 而非字符串匹配，避免对 error 文案的脆弱依赖（不同语言/版本
// 系统的 Windows 错误消息可能本地化不同）。goosFn 默认返回 runtime.GOOS，
// 便于测试不依赖 build tag / 跨平台执行环境即可断言平台行为。
//
// 平台守卫（关键）：仅在 GOOS == "windows" 时才判 errno，否则永远 false。
// 否则 Linux 上 errno=5(EIO) / errno=32(EPIPE) 也是合法 errno，会被误判为
// 「DLL 占用」而吞掉真实 I/O 错误，违反「不静默吞错」铁律。
func isPlatformLibBusy(err error) bool {
	if goosFn() != "windows" {
		return false
	}
	var sysErr syscall.Errno
	if !errors.As(err, &sysErr) {
		return false
	}
	return sysErr == errnoAccessDeniedWin || sysErr == errnoSharingViolationWin
}

// goosFn 暴露当前平台名供 isPlatformLibBusy 使用，默认走 runtime.GOOS。
// 注入点：测试可在用例内临时改成 "linux" / "windows" 等验证平台分支语义，
// 不依赖真实运行环境（避免跨 OS runner 行为差异）。
var goosFn = func() string { return runtime.GOOS }

// tempDirFn / readDirFn 是 sweepStaleTempDirs 的注入点，参考 removeDirFn / goosFn
// 的注入风格：默认走 os.TempDir / os.ReadDir，测试可在用例内替换为可控 mock。
var (
	tempDirFn = os.TempDir
	readDirFn = os.ReadDir
)

// sweepFn 是 sweepStaleTempDirs 自身的注入点，便于 extractModels 集成测试
// 拦截「helper 被调用」事件。默认走 sweepStaleTempDirs。
var sweepFn = sweepStaleTempDirs

// ocrTempPrefix 是 sweepStaleTempDirs 识别「本工具遗留临时目录」的唯一前缀。
// 宁可少删不可误删——前缀必须与 extractModels 的 os.MkdirTemp 模板完全一致。
const ocrTempPrefix = "nazhi-cli-ocr-"

// sweepStaleTempDirs 清理 os.TempDir() 下历史进程遗留的 nazhi-cli-ocr-* 目录。
//
// 调用时机：在 extractModels 成功建出当前目录之后、返回之前。
// currentDir 是本次新建的目录绝对路径，自身被跳过（避免误删正在使用的实例）。
//
// 行为约束（业界惯例，Chrome/VSCode 等同款）：
//   - best-effort：单个目录删不掉（如仍被其它运行中的实例 LoadLibrary 占用）
//     静默跳过，不中断、不冒泡为致命错误——这天然安全，占用中的目录 OS 会拒绝删除。
//   - 防误删：仅匹配 ocrTempPrefix 前缀的目录，其它程序目录（chrome-*、vscode-*、
//     go-build-* 等）、文件、非目录条目一律不碰。
//   - 失败不阻断：sweep 失败（如 ReadDir 失败）不影响 extractModels 的主流程返回值。
func sweepStaleTempDirs(currentDir string) error {
	entries, err := readDirFn(tempDirFn())
	if err != nil {
		// 读不到 %TEMP% 列表（权限拒绝、路径不存在等）静默返回：
		// 清扫是锦上添花，扫不到也不影响 OCR 初始化主路径。
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, ocrTempPrefix) {
			continue
		}
		// 必须按「目录」处理——os.MkdirTemp 创的是目录，但 %TEMP% 里可能混入
		// 同前缀的遗留文件（理论上不存在，但 IsDir 过滤可让 helper 防御性更强）。
		if !e.IsDir() {
			continue
		}
		fullPath := filepath.Join(tempDirFn(), name)
		// 跳过本次刚建的目录——可能仍有 goroutine 后续用到（虽然路径已绑定到 o.tempDir，
		// 但显式跳过更清晰，也避免一种边界：currentDir 由 MkdirTemp 返回、绝对路径
		// 与 ReadDir 拼接结果在大小写不敏感文件系统上不一致）。
		if currentDir != "" && strings.EqualFold(filepath.Clean(fullPath), filepath.Clean(currentDir)) {
			continue
		}
		// best-effort：删除失败（如 DLL 占用）静默跳过。
		_ = removeDirFn(fullPath)
	}
	return nil
}

// Windows errno 数值常量（避免依赖 syscall 包内 Windows-only 常量名，
// 方便在跨平台测试中模拟 Windows 行为）。
//
//	ERROR_ACCESS_DENIED       = 5
//	ERROR_SHARING_VIOLATION   = 32
const (
	errnoAccessDeniedWin     syscall.Errno = 5
	errnoSharingViolationWin syscall.Errno = 32
)

// ─── 进程级全局同步 ───

// SetOnnxRuntimePath 是进程级全局函数，Pool 多实例并发初始化时
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
	initMu      sync.Mutex  // 保护初始化路径和 closed 翻转
	initialized bool        // true = 初始化已完成（成功或失败由 initErr 决定）
	closed      atomic.Bool // true = Close() 已调用，禁止后续识别
	initErr     error
	ocr         *ddddocr.DdddOcr
	tempDir     string
	mu          sync.Mutex // 保护 Classification 调用，支持单例并发安全
	// testPanicHook 仅供内部测试触发 initOnce 内的 panic，
	// 生产代码中始终为 nil，对运行路径零影响。
	testPanicHook func()
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
// 启用并发：NewPool(n) 暗示期望 n 个独立 session 实例，具体预热由首次
// Recognize 的惰性初始化完成。如需提前预热，调 WarmUp。
// 内存代价：每个实例约 50MB（ONNX 模型 + 原生库解压到独立 tempDir），n=4 ≈ 200MB。
// 业务场景：批量调用 Login() 时才需要调高；单 Login 调一次用 1 实例足够。
//
// inits 字段用 sync.Map 存储已注册实例，原因：
//   - 99 次串行 Recognize = 99 次 trackInit(同一 *OCR)
//   - sync.Map.LoadOrStore 在 key 已存在时是 lock-free 路径，
//     避免 mutex.Lock + map 写入的固定开销
//   - sync.Map 读写并发安全，无需额外的 initsMu 保护
//   - Close 路径用 Range 迭代，配合 closeOnce 仍保证只跑一次 Close 工作
//
// 用 closeMu 保护整个「read closed + Get + trackInit」
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

// NewPool 创建 OCR 实例池。
// preload 参数保留以保持 API 向后兼容，不再同步预热 ONNX session。
// 惰性初始化在首次 Recognize 时触发。调用方可后续通过 WarmUp 异步预热。
func NewPool(preload int) *Pool {
	return &Pool{
		pool: sync.Pool{New: func() any { return &OCR{} }},
	}
}

// WarmUp 提前预热 n 个 OCR 惰性初始化（模型解压 + ONNX session 创建）。
// 首次 Recognize 也会自动惰性初始化，但程序启动后首次调用耗时 1-3s。
// 在后台 goroutine 提前调 WarmUp 可避免首次 Login 阻塞。
// ctx 用于取消长时间的解压操作；n <= 0 时 no-op。
// 预热成功的实例会被自动放回池中供 Recognize 复用。
func (p *Pool) WarmUp(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}
	for i := 0; i < n; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		o := &OCR{}
		// 直接触发生效模型解压 + ddddocr.New
		if err := o.initOnce(); err != nil {
			return err
		}
		p.trackInit(o)
		p.pool.Put(o)
	}
	return nil
}

// trackInit 记录首次完成惰性初始化的 OCR 实例。
// 用 sync.Map.LoadOrStore 实现「key 已存在时 lock-free 跳过」——
//
//	99 次串行 Recognize(同一 *OCR) 只触发 1 次实际 Store，
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
// close 检查 + pool.Get + trackInit 必须在同一 mutex 临界区内。
//
//	否则并发 Recognize 可穿过 Close 的 Range 完成窗口，向 inits map
//	注册新实例但 Close 已不会再次访问 -> tempDir 永久泄漏。
//	具体场景：
//	  T0 Close 进入 closeOnce -> 拿 closeMu -> Range(inits) -> 翻 closed -> 放 closeMu
//	  T1 Recognize 拿 closeMu（在 T0 之后）-> 看到 closed=true -> 直接返回错误
//	  T1' Recognize 拿 closeMu（在 T0 之前）-> 看到 closed=false -> Get + trackInit
//	       -> 放 closeMu -> Close 路径（拿 closeMu）-> Range 包含 T1' 注册的实例
//	保证：T1 要么被 Close 之前完整处理（被 Close 清理），要么被 Close 之后拒绝。
//
// o.Recognize 在 closeMu 外执行，但 OCR 级别有 atomic closed 二次检查
// （在 o.mu 临界区内、Classification 前），形成两层防御：
//   - 层 1（Pool）：closeMu 保证 trackInit 窗口不泄漏
//   - 层 2（OCR）：atomic closed 二次检查保证永不访问已关闭 session
func (p *Pool) Recognize(imageData []byte) (result string, err error) {
	// 临界区：close 检查 + Get + trackInit 原子完成
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
		return "", errors.New("OCR pool is closed")
	}

	// F5.3 修复：o.Recognize 内部 panic 时不 Put 回 pool（状态不明的实例
	// 可能含残缺的 tempDir / nil ocr 等），让 GC 回收。正常返回才归还。
	// 注意：initOnce 的 deferred recover 已清理 tempDir，不会泄漏。
	panicked := true
	defer func() {
		if panicked {
			_ = recover() // 吞掉，不 Put 回去
		} else {
			p.pool.Put(o)
		}
	}()
	result, err = o.Recognize(imageData)
	panicked = false
	return result, err
}

// Close 释放池中所有已完成惰性初始化的 OCR 实例 (ONNX session + 临时目录)。
//
// 注意：sync.Pool 持有的是结构体, 真正需要 Close 的是初始化过 session 的实例。
// 池内只跟踪首次完成初始化 (Recognize 后) 的实例, 避免漏释放或重复释放。
//
// 多实例池 (NewPool(N>1)) 下, 每个实例对应独立 tempDir, Close 会释放全部。
//
// 并发安全：用 sync.Once 保证"排空 map + 迭代 Close 实例"这一段
// 关键路径只跑一次。即使多个 goroutine 同时调 Close，第一次调用的协程
// 负责全部释放工作，后续调用立即返回 nil，避免同一实例被 Close 两次。
//
// closeMu 保护「Range(inits) + 翻 closed」原子临界区，与 Recognize
//
//	路径的「读 closed + Get + trackInit」临界区互斥。任何并发 Recognize 要么：
//	  1) 在 Close 临界区之前完成 trackInit -> 被 Range 清理
//	  2) 在 Close 临界区之后拿 closeMu -> 看到 closed=true -> 直接返回错误
//	不会有"漏网"的 trackInit 留下幽灵实例。
//
// Pool.inits 是 sync.Map（无独立 initsMu），Close 路径用 Range
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
//
// 保守方案：保留 wrapper（统一调用入口价值：调用点只需调 platformLibName()
// 而不必每次写 platformLibNameFor(runtime.GOOS)），加注释说明。
func platformLibName() string {
	return platformLibNameFor(runtime.GOOS)
}

// platformLibNameFor 是 platformLibName 的参数化版本，接受任意 GOOS 字符串。
// 补充 darwin 分支返回 .dylib 扩展名。
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

// initOnce 在 initMu 锁定状态下执行 OCR 实例的惰性初始化。
// deferred recover 捕获 extractModels/ddddocr.New 的 panic，
// 清理 tempDir + 保留 panic 根因到 initErr + 不标记 initialized，
// 防止 tempDir 永久泄漏到 %TEMP% 且允许后续 Recognize 重试。
func (o *OCR) initOnce() (retErr error) {
	// deferred recover 捕获 initMu 临界区内 panic，
	// 清理 tempDir + 保留根因到 initErr
	var cleanupTempDir bool
	defer func() {
		if r := recover(); r != nil {
			if cleanupTempDir && o.tempDir != "" {
				_ = os.RemoveAll(o.tempDir)
				o.tempDir = ""
			}
			// 输出 stack trace 到 stderr，避免 CGO 层 panic 的堆栈永久丢失
			fmt.Fprintf(os.Stderr, "initOnce panic stack:\n%s\n", debug.Stack())
			// 保留 panic 根因到 initErr，不标记 initialized，
			// 让后续 Recognize 重试 initOnce 并把根因上报，
			// 避免 *OCR 实例被「initialized=true + initErr=nil + ocr=nil」永久卡死。
			o.initialized = false
			if err, ok := r.(error); ok {
				o.initErr = fmt.Errorf("initOnce panic: %w", err)
			} else {
				o.initErr = fmt.Errorf("initOnce panic: %v", r)
			}
			retErr = o.initErr
		}
	}()

	o.tempDir, o.initErr = o.extractModels()
	if o.initErr != nil {
		o.initialized = true
		return fmt.Errorf("OCR initialization failed: %w", o.initErr)
	}
	cleanupTempDir = true

	// SetOnnxRuntimePath + ddddocr.New 用 initMuGlobal 保护，
	// 确保多实例并发初始化时 SetOnnxRuntimePath 和 New 不被交叉覆盖。
	// onceSetPath 确保 SetOnnxRuntimePath 在整个进程中只调用一次。
	initMuGlobal.Lock()
	libPath := filepath.Join(o.tempDir, platformLibName())
	defer initMuGlobal.Unlock()
	onceSetPath.Do(func() {
		ddddocr.SetOnnxRuntimePath(libPath)
	})

	// 创建识别器，指定模型目录为解压目录
	// testPanicHook 只触发一次，防止重试时无限 panic+recover 循环（F3）
	if o.testPanicHook != nil {
		hook := o.testPanicHook
		o.testPanicHook = nil
		hook()
	}
	opts := ddddocr.DefaultOptions()
	opts.ModelDir = o.tempDir
	ocr, err := ddddocr.New(opts)
	if err != nil {
		_ = os.RemoveAll(o.tempDir)
		o.tempDir = ""
		o.initErr = fmt.Errorf("创建 ddddocr 失败: %w", err)
		o.initialized = true
		return o.initErr
	}

	// 限制字符范围为 大写+小写+数字（验证码通常包含这些）
	ocr.SetRanges(ddddocr.RangeLowerUpperDigit)

	o.ocr = ocr
	o.initialized = true
	cleanupTempDir = false // 初始化成功，不再需要清理
	return nil
}

// Recognize 对图片字节进行验证码识别，返回识别出的文本。
// imageData 应为 JPEG 或 PNG 编码的字节。
func (o *OCR) Recognize(imageData []byte) (string, error) {
	// 用 initMu 保护整个初始化路径，确保多 goroutine 下也只解压一次。
	// 关键设计：Close() 会把 closed=true、initErr=nil、initialized=false、
	// o.ocr=nil，之后调 Recognize 走「重新初始化」分支（除非已 close）。
	//
	// 使用 defer Unlock 防止 initOnce 内部 panic（recover 路径）后 initMu 永不解锁。
	// 场景：initOnce 内 onnxruntime 初始化崩溃 → deferred recover 清理 tempDir +
	// 把 panic 根因写入 initErr + 不重新抛出 → defer Unlock 仍能执行。
	// 之前版本会 panic(r) 重新抛出，依赖 defer Unlock 才能正确释放锁；
	// 现在 panic 被吞入 initErr，defer Unlock 也保证锁释放。
	o.initMu.Lock()
	defer o.initMu.Unlock()
	if o.closed.Load() {
		return "", errors.New("OCR is closed")
	}
	if !o.initialized {
		if err := o.initOnce(); err != nil {
			return "", err
		}
	}
	initErr := o.initErr
	ocr := o.ocr

	if initErr != nil {
		return "", fmt.Errorf("OCR initialization failed: %w", initErr)
	}
	// 防御：initialized=true 但 ocr=nil 是不一致状态（Close 后、初始化中途异常等）
	if ocr == nil {
		return "", errors.New("OCR unavailable: recognizer is nil")
	}

	// 单例场景下保护 Classification 调用，并发请求时串行执行识别
	// Classification 前 atomic 二次检查 closed。
	// 场景：T0 拿到 o.mu 进入 Classification 阻塞 → T1 Close 翻 closed + 关闭 ddddocr session
	//  → T0 持已关闭 session → ddddocr C 运行时 segfault。
	// 持 o.mu 后 Load closed 是无锁快速路径，Close 后立即返回错误，永不调 Classification。
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed.Load() {
		return "", errors.New("OCR is closed")
	}
	result, err := ocr.Classification(imageData)
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
//
// closed 改 atomic.Bool，Close 内先 Store(true)，让所有持 o.mu 阻塞在
//
//	Classification 之前的 goroutine 立即在二次检查中失败（永不访问已关闭 ddddocr
//	session）。这是 use-after-close 窗口的修复核心。
func (o *OCR) Close() error {
	o.initMu.Lock()
	o.closed.Store(true)
	o.initialized = false
	o.initErr = nil
	ocr := o.ocr
	o.ocr = nil
	tempDir := o.tempDir
	o.tempDir = ""
	o.initMu.Unlock()

	var errs []error
	if ocr != nil {
		o.mu.Lock()
		if err := ocr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭 ddddocr 引擎: %w", err))
		}
		o.mu.Unlock()
	}
	if tempDir != "" {
		if err := cleanupTempDir(tempDir); err != nil {
			errs = append(errs, err)
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

	// 顺手 best-effort 清扫 %TEMP% 下历史进程遗留的 nazhi-cli-ocr-* 目录。
	// 本次新建的 dir 会被显式跳过；sweep 失败不影响本次初始化。
	_ = sweepFn(dir)

	return dir, nil
}

// writeModelFile 写入模型文件并设置 0644 权限。
// 写入失败时走 cleanupTempDir 清理临时目录（复用 Windows DLL 占用降级逻辑）。
// 提取为 helper，消除三次 writeFile + cleanupTempDir + fmt.Errorf 的重复模式。
func writeModelFile(dir, name string, data []byte) error {
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		_ = cleanupTempDir(dir)
		return fmt.Errorf("写入 %s 失败: %w", name, err)
	}
	return nil
}
