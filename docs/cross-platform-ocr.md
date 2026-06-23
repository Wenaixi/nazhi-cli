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
1. `os.MkdirTemp()` 创建临时目录
2. 写入 onnxruntime 库（按平台命名）
3. `ddddocr.SetOnnxRuntimePath(libPath)`
4. 加载 ONNX 模型 + 字符集
5. 清理时 `os.RemoveAll(tempDir)`

## 进程级单例

`ocr.GetDefault()` 进程共享一个 OCR 引擎：

```go
var (
    defaultOCR *OCR
    defaultOnce sync.Once
)

func GetDefault() *OCR {
    defaultOnce.Do(func() {
        defaultOCR = &OCR{}
    })
    return defaultOCR
}
```

- 多个 `client.New()` 共享同一 `*OCR` 实例
- 模型只解压一次（约 14 MB → 临时目录）
- 内部 `sync.Mutex` 保证并发安全
- 99 次重试机制（同一张图片）提高识别准确率

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
