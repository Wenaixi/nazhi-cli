# 跨平台 OCR 设计

## 挑战

`go-ddddocr` 依赖 `onnxruntime_go`，后者在 Linux/macOS 强制 CGO，无法从其他 OS 交叉编译。每个平台需要原生库 + build tag 隔离。

## 支持的平台

| GOOS | GOARCH | 状态 | 备注 |
|------|--------|------|------|
| windows | amd64 | 支持 | 主力测试平台 |
| windows | arm64 | 支持 | Windows on ARM（zig cc 交叉编译）|
| linux | amd64 | 支持 | 服务器主流 |
| linux | arm64 | 支持 | ARM 服务器 / Raspberry Pi 4+ |
| darwin | arm64 | 支持 | Apple Silicon |
| darwin | amd64 | 不支持 | Microsoft 已停发 macOS x86_64 onnxruntime |

## 文件结构

```
internal/ocr/
├── ocr.go                       # 进程级单例 + 业务封装
├── onnx_win_amd64.go            //go:build windows && amd64
├── onnx_win_arm64.go            //go:build windows && arm64
├── onnx_lin_amd64.go            //go:build linux && amd64
├── onnx_lin_arm64.go            //go:build linux && arm64
├── onnx_mac_arm64.go            //go:build darwin && arm64
└── models/
    ├── common_old.onnx          # ONNX 模型（跨平台，14 MB）
    ├── charsets_old.json        # OCR 字符集（跨平台，57 KB）
    ├── onnxruntime_win_amd64.dll
    ├── onnxruntime_win_arm64.dll
    ├── libonnxruntime_lin_amd64.so
    ├── libonnxruntime_lin_arm64.so
    └── libonnxruntime_mac_arm64.dylib
```

## build tag 隔离

```go
// onnx_win_amd64.go
//go:build windows && amd64
package ocr

import _ "embed"

//go:embed models/onnxruntime_win_amd64.dll
var OnnxRuntimeDLL []byte
```

```go
// onnx_lin_amd64.go
//go:build linux && amd64
package ocr

import _ "embed"

//go:embed models/libonnxruntime_lin_amd64.so
var OnnxRuntimeDLL []byte
```

Go 编译时按 `(GOOS, GOARCH)` 只取对应文件嵌入。

## 为什么必须解压到磁盘

`go-ddddocr` → `onnxruntime_go` v1.25.0 强制要磁盘路径（C 运行时 `dlopen` / `LoadLibrary` 不支持内存模块）。

`ocr.go` 启动时：
1. `os.MkdirTemp()` 创建临时目录（`nazhi-cli-ocr-*` 前缀）
2. 写入 onnxruntime 库（按平台命名）
3. `ddddocr.SetOnnxRuntimePath(libPath)`
4. 加载 ONNX 模型 + 字符集
5. **v0.4.0 新增**：best-effort 清扫 `%TEMP%` 下历史进程遗留的同前缀目录（见下文）
6. 清理时 `os.RemoveAll(tempDir)`（**v0.4.0 新增**：Windows DLL 占用降级，见下文）

## 进程级单例

历史上 `ocr.GetDefault()` 提供进程级单例，多个 `client.New()` 共享同一个
`*OCR` 实例，模型只解压一次（约 14 MB → 临时目录），内部 `sync.Mutex`
保证并发安全。

**v0.3.4+ 变更**：删除 `ocr.GetDefault()` 0 调用方的进程级单例 API。
现在多个 Client 通过 `Pool` 实例共享引擎——`pkg/client` 的 `client.New`
默认构造一个 `ocr.NewPool(0)`（懒加载单实例），业务代码无需关心单例。

## OCR 可选构建（v0.3.5+）

v0.3.5 起，OCR 引擎通过 Go build tags 可选启用：

| 构建方式 | 命令 | OCR 行为 |
|---------|------|---------|
| 含 OCR（默认）| `go build -tags ddddocr -o nazhi.exe ./cmd/nazhi` | 内嵌 ddddocr 引擎，开箱即用 |
| 纯 Go 无 OCR | `go build -o nazhi-noocr.exe ./cmd/nazhi` | CGO-free 构建，Login 需要 WithCustomOCR |

**设计动机**：
- Nazhi-auto 等消费者需要 CGO_ENABLED=0 构建（无法引入 ddddocr 的 CGO 依赖）
- `!ddddocr` 构建下 `defaultOCR()` 返回 nil，`Login()` 立即返回 `ErrOCRNotConfigured`
- 调用方需通过 `WithCustomOCR` 注入自定义识别器
- `WithOCRConcurrency` 在 `!ddddocr` 构建下为 no-op warn（不会 panic 或替换已有 ocr）

**pkg/client 新增文件**：
- `client_ocr_enabled.go` — `//go:build ddddocr`，`defaultOCR()` 返回 `ocr.NewPool(0)`
- `client_ocr_disabled.go` — `//go:build !ddddocr`，`defaultOCR()` 返回 nil，`WithOCRConcurrency` 为占位 warn

## v0.4.0 三轮 OCR 修复

### 轮次 A：Windows DLL 占用降级（commit 5ff0ea8）

**问题**：在 Windows 上执行 `nazhi login`（`-tags=ddddocr` 构建）后，`Pool.Close` →
`OCR.Close` 调 `os.RemoveAll` 删临时目录时，`onnxruntime.dll` 仍被 CGO `LoadLibrary` 持锁，
Windows 在进程退出前不会释放该 DLL 的 mmap 文件句柄。`RemoveAll` 命中 `onnxruntime.dll`
返回 `ERROR_ACCESS_DENIED(5)` / `ERROR_SHARING_VIOLATION(32)`，被 `Pool.Close` 并入返回值，
污染 stderr：

> "关闭 Client 资源失败: 关闭 OCR 识别器: 清理临时目录 ...: unlinkat ...\onnxruntime.dll: Access is denied."

**修复**（`internal/ocr/ocr.go`）：
- 抽 `cleanupTempDir` helper，注入 `removeDirFn` 函数变量便于测试 mock
- 用 `errors.As` 判定 `syscall.Errno`，仅对 Windows 两个占用类 errno（5 / 32）降级返回 nil
- 其他错误（Linux EPERM、磁盘满、只读卷、路径不存在）照常返回，**不静默吞错**

### 轮次 B：GOOS=windows 平台守卫（commit a81c9f3）

**问题**：5ff0ea8 引入的 `isPlatformLibBusy` 仅判 `syscall.Errno` 数值（5 / 32），注释承诺
「非 Windows 永远 false」但代码未做平台守卫。Linux 上 `EIO=5` / `EPIPE=32` 也是合法 errno，
会被误判为「DLL 占用」而吞掉真实 I/O 错误，违反「不静默吞错」铁律。

**修复**：
- 抽 `goosFn` 函数变量作为平台注入点，默认 `runtime.GOOS`，便于测试不依赖 build tag
- `isPlatformLibBusy` 入口加 `goosFn() != "windows"` 守卫，非 Windows 直接返 false
- `cleanupTempDir` doc 块新增「跨平台」段落，明确降级语义只在 Windows 生效

### 轮次 C：启动清扫历史 temp 目录（commit 7d5dd65）

**问题**：每次 `extractModels` 会用 `os.MkdirTemp("", "nazhi-cli-ocr-*")` 建一个唯一新目录。
Windows 上 `onnxruntime.dll` 被 CGO `LoadLibrary` 占用，`Close()` 删不掉同进程的 `tempDir`
（已有上轮降级处理），进程退出后句柄释放，旧目录才能被下次进程清掉——累积到 966+ 个，约 3.8MB。

**修复**：业界惯例（Chrome / VSCode 同款）在新建临时目录时顺手 best-effort 清扫 `%TEMP%`
下历史进程遗留的同前缀目录：

- 新增 `sweepStaleTempDirs(currentDir)` helper，注入点 `tempDirFn / readDirFn / removeDirFn / sweepFn`
- `extractModels` 在 dir 建好后调用 `sweepFn(dir)`，best-effort（`_ = ...`）
- **本进程新建的目录被显式跳过**（避免误删正在使用的实例）
- **删不掉的静默跳过**（如仍被其它运行中的实例 `LoadLibrary` 占用）
- **防误删**：仅匹配 `ocrTempPrefix` 前缀的目录，其它程序目录 / 文件 / 非目录条目一律不碰

测试覆盖（5 个新用例）：保留 current / 不碰其它程序目录 / 单个删失败不阻断 / `ReadDir` 失败返回 nil /
`TestExtractModels_CallsSweepAfterMkdirTemp` 集成断言（确保 helper 真的被 `extractModels` 调用）。

### 三轮修复的注入点风格

```go
// 默认走标准库函数，测试可在用例内替换为可控 mock
var (
    removeDirFn = os.RemoveAll        // cleanupTempDir 注入点
    tempDirFn   = os.TempDir          // sweepStaleTempDirs 注入点
    readDirFn   = os.ReadDir          // sweepStaleTempDirs 注入点
    sweepFn     = sweepStaleTempDirs  // extractModels 拦截点（集成测试用）
    goosFn      = func() string { return runtime.GOOS }  // 平台注入点
)
```

Windows errno 数值常量（避免依赖 syscall 包内 Windows-only 常量名）：

```go
const (
    errnoAccessDeniedWin     syscall.Errno = 5   // ERROR_ACCESS_DENIED
    errnoSharingViolationWin syscall.Errno = 32  // ERROR_SHARING_VIOLATION
)
```

## CI 矩阵

`onnxruntime_go` 在 Linux/macOS 强制 CGO，无法从其他 OS 交叉编译。CI 每个平台用 native runner（或 cross-compile 工具链）：

| 平台 | Runner | CGO_ENABLED | 编译方式 |
|------|--------|-------------|----------|
| Linux amd64 | `ubuntu-latest` | 1 | Native |
| Linux arm64 | `ubuntu-latest` | 1 | Cross-compile: `gcc-aarch64-linux-gnu` |
| macOS arm64 | `macos-latest` | 1 | Native |
| Windows amd64 | `windows-latest` | 1 | Native MinGW |
| Windows arm64 | `windows-latest` | 1 | Cross-compile: `zig cc -target aarch64-windows-gnu` |

Linux arm64 用 `gcc-aarch64-linux-gnu` 在 amd64 runner 上交叉编译（不依赖稀缺的 ARM64 runner 资源池）。
Windows arm64 用 `zig cc` 自带 aarch64-windows-gnu 工具链，绕过 MinGW gcc 只出 x86_64 的限制。

## 文件命名规则

| GOOS | 文件名 |
|------|--------|
| windows | `onnxruntime.dll` |
| linux | `libonnxruntime.so` |
| darwin | `libonnxruntime.dylib` |

`platformLibName()` 根据 `runtime.GOOS` 动态返回解压时的目标文件名。

## 下载源

从 [Microsoft onnxruntime v1.25.0 releases](https://github.com/microsoft/onnxruntime/releases/tag/v1.25.0) 下载，与 `onnxruntime_go` v1.25.0 ABI 对齐。

## Windows 杀软误报

内嵌的 `onnxruntime.dll` 是 Microsoft 官方二进制，部分杀软会误报。这是 go-ddddocr + onnxruntime 生态的通用问题。建议：
- 在白名单中添加本程序
- 在企业内网环境使用
- 使用代码签名证书签名

## macOS x86_64 不支持

Microsoft onnxruntime v1.25.0 已停止发布 macOS x86_64 版本（Apple 全面转向 Silicon）。本项目不打算支持。如需 Intel Mac 请使用 v1.20.x 的 onnxruntime（需自行 fork OCR 库）。
