# 跨平台 OCR 设计

## 挑战

`go-ddddocr` 依赖 `onnxruntime_go`，后者在 Linux/macOS 强制 CGO，无法从其他 OS 交叉编译。每个平台需要原生库 + build tag 隔离。

加上验证码识别对内存与启动速度敏感（每次 login 都跑），必须把 ONNX 模型 + 原生库嵌入二进制、`go:embed` 一次到位。

## 支持的平台

| GOOS | GOARCH | 状态 | 备注 |
|---|---|---|---|
| windows | amd64 | ✅ | 主力测试平台 |
| windows | arm64 | ✅ | Windows on ARM（zig cc 交叉编译） |
| linux | amd64 | ✅ | 服务器主流 |
| linux | arm64 | ✅ | ARM 服务器 / Raspberry Pi 4+ |
| darwin | arm64 | ✅ | Apple Silicon |
| darwin | amd64 | ❌ **不支持** | Microsoft 已停发 macOS x86_64 onnxruntime |

## 文件结构

```
internal/ocr/
├── ocr.go                          # OCR Pool + 跨平台路径处理 + Windows DLL 降级 + 启动清扫
├── ocr_sweep_test.go               # 启动清扫测试（v0.4.0 第 3 轮修复）
├── ocr_win_cleanup_test.go         # Windows DLL 降级测试（v0.4.0 第 1 轮修复）
├── onnx_win_amd64.go               //go:build windows && amd64
├── onnx_win_arm64.go               //go:build windows && arm64
├── onnx_lin_amd64.go               //go:build linux && amd64
├── onnx_lin_arm64.go               //go:build linux && arm64
├── onnx_mac_arm64.go               //go:build darwin && arm64
└── models/
    ├── common_old.onnx             # ONNX 模型（跨平台，约 14 MB）
    ├── charsets_old.json           # OCR 字符集（跨平台，约 57 KB）
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

Go 编译时按 `(GOOS, GOARCH)` 只取对应文件嵌入。其他平台编译时该 `var OnnxRuntimeDLL` 不存在（编译报错？**不会**——因为每个文件只在该 build tag 下被编译，其他平台下整文件被剪掉，引用该变量的代码 `extractModels` 也要分平台）。

实际 `extractModels` 的 `runtime.GOOS` 分支只引用该平台下存在的 `OnnxRuntimeDLL`：

```go
platformLibName := func() string {
    switch runtime.GOOS {
    case "windows":
        return "onnxruntime.dll"
    case "linux":
        return "libonnxruntime.so"
    case "darwin":
        return "libonnxruntime.dylib"
    default:
        return ""
    }
}
```

## 为什么必须解压到磁盘

`go-ddddocr` → `onnxruntime_go` v1.25.0 强制要磁盘路径（C 运行时 `dlopen` / `LoadLibrary` 不支持内存模块）。

`ocr.go` 启动时：

```
1. os.MkdirTemp("", "nazhi-cli-ocr-*") 建临时目录
2. 写 onnxruntime 库（按 platformLibName 命名）
3. ddddocr.SetOnnxRuntimePath(libPath)
4. 加载 ONNX 模型 + 字符集
5. sweepFn(tempDir)  best-effort 清扫其他历史进程的 nazhi-cli-ocr-* 残留
6. Close 时 os.RemoveAll(tempDir)（Windows DLL 占用降级，见下）
```

## Pool 多实例 + sync.Mutex

v0.4.0 起 OCR 用 `Pool` 多实例替代 v0.3.5 之前的进程级单例（`GetDefault()` 已删除）：

```go
pool := ocr.NewPool(0)  // 0 = 懒加载 1 实例
pool := ocr.NewPool(4)  // 预热 4 个实例
```

**Pool 设计**：
- 基于 `sync.Pool` 复用实例
- 默认 `NewPool(0)` 懒加载 1 实例 + `sync.Mutex` 串行化
- `WithOCRConcurrency(n)` 预热 n 个独立 ONNX session（各约 50MB）

**内存代价**：N=4 约 200MB（4 实例 × 50MB）。业务场景批量 Login 才需要调高；单次 Login 用默认足够。

> **历史说明**：v0.3.4 之前曾提供 `ocr.GetDefault()` 进程级单例，但生产代码无调用方，已删除。

## OCR 可选构建（v0.3.5+）

```go
// client_ocr_enabled.go  //go:build ddddocr
func defaultOCR() CaptchaRecognizer {
    return ocr.NewPool(0)  // 懒加载 1 实例
}

// client_ocr_disabled.go  //go:build !ddddocr
func defaultOCR() CaptchaRecognizer {
    return nil  // 调用方必须 WithCustomOCR
}
```

| 构建方式 | 命令 | OCR 行为 |
|---|---|---|
| **含 OCR（默认 release）** | `go build -tags ddddocr -o nazhi.exe ./cmd/nazhi` | 内嵌 ddddocr 引擎，开箱即用 |
| **纯 Go 无 OCR** | `go build -o nazhi-noocr.exe ./cmd/nazhi` | CGO-free，`Login()` 立即返 `ErrOCRNotConfigured` |

**设计动机**：Nazhi-auto 等嵌入式消费者需要 `CGO_ENABLED=0` 构建（无法引入 ddddocr 的 CGO 依赖）。
- `!ddddocr` 构建下 `defaultOCR()` 返 nil
- `Login()` 立即返 `ErrOCRNotConfigured`，错误消息明确给出 actionable 指引（用预编译 release 或 `WithCustomOCR` 注入）
- `WithOCRConcurrency` 在 `!ddddocr` 构建下为 no-op warn（不会 panic 也不会替换已有 ocr）

## Windows OCR 三轮修复（v0.4.0 第 15/16 轮 TDD）

### 轮次 A：Windows DLL 占用降级（commit `5ff0ea8`）

**问题**：Windows 上 `nazhi login`（`-tags=ddddocr` 构建）后，`Pool.Close` → `OCR.Close` 调 `os.RemoveAll` 删临时目录时，`onnxruntime.dll` 仍被 CGO `LoadLibrary` 持锁，Windows 在进程退出前不会释放该 DLL 的 mmap 文件句柄。`RemoveAll` 命中 `onnxruntime.dll` 返回 `ERROR_ACCESS_DENIED(5)` / `ERROR_SHARING_VIOLATION(32)`，被 `Pool.Close` 并入返回值，污染 stderr：

> "关闭 Client 资源失败: 关闭 OCR 识别器: 清理临时目录 ...: unlinkat ...\onnxruntime.dll: Access is denied."

**修复**（`internal/ocr/ocr.go`）：
- 抽 `cleanupTempDir` helper，注入 `removeDirFn` 函数变量便于测试 mock
- 用 `errors.As` 判定 `syscall.Errno`，仅对 Windows 两个占用类 errno（5 / 32）降级返 nil
- 其他错误（Linux EPERM、磁盘满、只读卷、路径不存在）照常返回，**不静默吞错**

**新增测试**：`ocr_win_cleanup_test.go` 用 `removeDirFn = func() error { return syscall.Errno(5) }` 注入 Windows errno 5，断言不返 error。

### 轮次 B：GOOS=windows 平台守卫（commit `a81c9f3`）

**问题**：5ff0ea8 引入的 `isPlatformLibBusy` 仅判 `syscall.Errno` 数值（5 / 32），注释承诺「非 Windows 永远 false」但代码未做平台守卫。Linux 上 `EIO=5` / `EPIPE=32` 也是合法 errno，会被误判为「DLL 占用」而吞掉真实 I/O 错误，违反「不静默吞错」铁律。

**修复**：
- 抽 `goosFn` 函数变量作为平台注入点，默认 `runtime.GOOS`，便于测试不依赖 build tag
- `isPlatformLibBusy` 入口加 `goosFn() != "windows"` 守卫，非 Windows 直接返 false
- `cleanupTempDir` doc 块新增「跨平台」段落，明确降级语义只在 Windows 生效

**新增测试**：

```go
// Linux errno 5（EIO）模拟，断言不被误判
goosFn = func() string { return "linux" }
removeDirFn = func() error { return syscall.Errno(5) }
err := cleanupTempDir(dir)
// err != nil  ← 真的 I/O 错误，没被吞
```

### 轮次 C：启动时清扫历史 temp 目录（commit `7d5dd65`）

**问题**：每次 `extractModels` 会用 `os.MkdirTemp("", "nazhi-cli-ocr-*")` 建一个唯一新目录。Windows 上 `onnxruntime.dll` 被 CGO `LoadLibrary` 占用，`Close()` 删不掉同进程的 `tempDir`（已有上轮降级处理），进程退出后句柄释放，旧目录才能被下次进程清掉——累积到 966+ 个，约 3.8MB 残余 `%TEMP%` 占用。

**修复**：业界惯例（Chrome / VSCode 同款）在新建临时目录时顺手 best-effort 清扫 `%TEMP%` 下历史进程遗留的同前缀目录：

- 新增 `sweepStaleTempDirs(currentDir)` helper，注入点 `tempDirFn / readDirFn / removeDirFn / sweepFn`
- `extractModels` 在 dir 建好后调用 `sweepFn(dir)`，best-effort（`_ = ...`）
- **本进程新建的目录被显式跳过**（避免误删正在使用的实例）
- **删不掉的静默跳过**（如仍被其它运行中的实例 `LoadLibrary` 占用）
- **防误删**：仅匹配 `ocrTempPrefix` 前缀的目录，其它程序目录 / 文件 / 非目录条目一律不碰

**新增测试**（5 个）：
- `TestSweepStaleTempDirs_KeepsCurrent` — 不删当前 dir
- `TestSweepStaleTempDirs_IgnoresOtherPrograms` — 不碰 `chromedp-*` / `vscode-*` 等其它前缀
- `TestSweepStaleTempDirs_SingleFailureNotFatal` — 单个删除失败不阻断
- `TestSweepStaleTempDirs_ReadDirError` — ReadDir 失败返 nil（不是 fatal）
- `TestExtractModels_CallsSweepAfterMkdirTemp` — 集成断言，helper 真被调用

### 三轮修复的注入点风格

```go
// 默认走标准库函数，测试可在用例内替换为可控 mock
var (
    removeDirFn = os.RemoveAll          // cleanupTempDir 注入点
    tempDirFn   = os.TempDir            // sweepStaleTempDirs 注入点
    readDirFn   = os.ReadDir            // sweepStaleTempDirs 注入点
    sweepFn     = sweepStaleTempDirs    // extractModels 拦截点（集成测试用）
    goosFn      = func() string { return runtime.GOOS }  // 平台注入点
)
```

Windows errno 数值常量（避免依赖 syscall 包内 Windows-only 常量名）：

```go
const (
    errnoAccessDeniedWin     syscall.Errno = 5    // ERROR_ACCESS_DENIED
    errnoSharingViolationWin syscall.Errno = 32   // ERROR_SHARING_VIOLATION
)
```

## CI 矩阵

`onnxruntime_go` 在 Linux/macOS 强制 CGO，无法从其他 OS 交叉编译。CI 每个平台用 native runner 或 cross-compile 工具链：

| 平台 | Runner | CGO_ENABLED | 编译方式 |
|---|---|---|---|
| Linux amd64 | `ubuntu-latest` | 1 | Native |
| Linux arm64 | `ubuntu-latest` | 1 | Cross-compile: `gcc-aarch64-linux-gnu` |
| macOS arm64 | `macos-latest` | 1 | Native |
| Windows amd64 | `windows-latest` | 1 | Native MinGW |
| Windows arm64 | `windows-latest` | 1 | Cross-compile: `zig cc -target aarch64-windows-gnu` |

Linux arm64 用 `gcc-aarch64-linux-gnu` 在 amd64 runner 上交叉编译（不依赖稀缺的 ARM64 runner 资源池）。
Windows arm64 用 `zig cc` 自带 aarch64-windows-gnu 工具链，绕过 MinGW gcc 只出 x86_64 的限制。

## 文件命名规则

| GOOS | 文件名 |
|---|---|
| windows | `onnxruntime.dll` |
| linux | `libonnxruntime.so` |
| darwin | `libonnxruntime.dylib` |

`platformLibName()` 根据 `runtime.GOOS` 动态返回解压时的目标文件名。

## 下载源

从 [Microsoft onnxruntime v1.25.0 releases](https://github.com/microsoft/onnxruntime/releases/tag/v1.25.0) 下载，与 `onnxruntime_go` v1.25.0 ABI 对齐。

## macOS x86_64 不支持

Microsoft onnxruntime v1.25.0 已停止发布 macOS x86_64 版本（Apple 全面转向 Silicon）。本项目不打算支持。如需 Intel Mac，请：

- 使用 Linux x86_64 + Docker（推荐 `docker run --rm -v $(pwd):/work golang:1.26` 编译）
- 或 fork `yangbin1322/go-ddddocr` 改用 v1.20.x 的 onnxruntime
- 或干脆买台 Apple Silicon Mac（ARM Mac mini 起售价已较低）

## Windows 杀软误报

内嵌的 `onnxruntime.dll` 是 Microsoft 官方二进制，但部分杀软（特别是某些国产 AV）会因启发式扫描误报。
这是 `go-ddddocr` + onnxruntime 生态的通用问题，并非本项目特有的供应链问题。

**缓解方案**：
- 在白名单中添加本程序路径
- 在企业内部环境使用
- 使用代码签名证书签名（EV OV 证书均可消除大部分误报）

## 部署 / 调试技巧

### 看当前 ONNX 实例数

```go
c, _ := client.New(client.WithOCRConcurrency(4))
// 默认 min(4, NumCPU) ≈ 4 实例
```

`Pool.Len()` 在 debug 时可输出当前实例数 / 空闲实例数。

### OCR 慢的原因定位

```bash
$ NAZHI_TIMEOUT=60 nazhi -v login -u x -p y
[verbose] → GET https://www.nazhisoft.com/kaptcha/kaptcha.jpg?seq=1
[verbose] OCR 识别完成（4 字符）
[verbose] → POST https://www.nazhisoft.com/uiStudentLogin/validateCaptcha
[verbose] → POST https://www.nazhisoft.com/teacher/auth/studentLogin/validate
```

如果 OCR 阶段慢，多半是：
1. 单图识别本身慢（`Pool` 内部 `Recognize` 阻塞）—— 检查 CPU 占用、杀软扫描 DLL
2. 服务端 captcha 服务慢（kaptcha.jpg 图片下载）—— 加大 `NAZHI_TIMEOUT`
3. 99 张图都识别失败（ddddocr 模型错位）—— 升级 `internal/ocr/models/` 的模型文件

### 临时目录堆积问题（v0.4.0 修复前）

修复前 Windows 上 `nazhi login` N 次会留 N 个 `nazhi-cli-ocr-*` 目录在 `%TEMP%`。
修复后 `nazhi login` 会顺手 best-effort 清扫旧目录，**不必手动清理**。

如确实要手动清理（CI 流水线空间压力）：

```bash
# Linux / macOS
rm -rf /tmp/nazhi-cli-ocr-*

# Windows PowerShell
Remove-Item -Recurse -Force $env:TEMP\nazhi-cli-ocr-*
```

### CGO-free 部署

```bash
# 跨平台编译无 OCR 变体
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o nazhi-noocr-linux ./cmd/nazhi

# 运行时需要外部 OCR 服务（如 Tesseract、自研 API）
c, _ := client.New(
    client.WithCustomOCR(myExternalOCR),
)
```
