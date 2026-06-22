# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

一站式命令行工具 + Go SDK，用于纳智综合评价系统的自动化操作。提供 **SSO 登录**、**任务管理**、**自我评价**、**文件上传** 等完整功能。

✨ **特色**

- 🔐 **全自动 OCR 验证码** — 模型已嵌入二进制，无需下载、无需配置，开箱即用
- 🌍 **跨平台支持** — Windows / Linux / macOS（5 个平台 × 架构组合），单二进制运行
- 📦 **进程级 OCR 单例** — 多实例共享引擎，避免重复解压模型
- 🛠️ **CLI + SDK 双形态** — 脚本可直接调用，集成方导入 Go 包
- 🧪 **完整测试覆盖** — 单元测试 + race 检测 + 跨平台 CI + 真实环境集成测试

---

## 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [环境变量](#环境变量)
- [命令参考](#命令参考)
- [跨平台支持](#跨平台支持)
- [作为 Go SDK 使用](#作为-go-sdk-使用)
- [架构总览](#架构总览)
- [开发](#开发)
- [常见问题](#常见问题)
- [协议](#协议)

---

## 安装

### 预编译二进制（推荐）

从 [Releases](https://github.com/Wenaixi/nazhi-cli/releases) 下载对应平台的最新版本：

| 平台 | 架构 | 文件 |
|---|---|---|
| Windows | amd64 / arm64 | `nazhi-windows-amd64.exe` / `nazhi-windows-arm64.exe` |
| Linux | amd64 / arm64 | `nazhi-linux-amd64` / `nazhi-linux-arm64` |
| macOS | arm64 (Apple Silicon) | `nazhi-darwin-arm64` |

> macOS 仅支持 arm64（Apple Silicon），因为 Microsoft 已停发 onnxruntime macOS x86_64。

### go install

```bash
go install github.com/Wenaixi/nazhi-cli/cmd/nazhi@latest
```

### 从源码构建

```bash
git clone https://github.com/Wenaixi/nazhi-cli.git
cd nazhi-cli
make build         # 当前平台
make release       # 全平台
```

---

## 快速开始

### 1. 登录获取 Token

```bash
nazhi login -u 学号 -p 密码
```

输出（JSON 格式）：

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9.eyJzdWIiOi...",
  "refresh_after": "...",
  "expires_at": "...",
  "user_info": null
}
```

> 🔒 **安全提示**：Token 等同于密码，请妥善保管。脚本中可使用 `--token` 参数或从环境变量传入。

### 2. 激活业务 Session（使用 Token）

```bash
nazhi session activate --token "eyJhbGciOiJIUzUxMiJ9..."
```

> Session 激活后会保持服务端状态（Cookie），后续 API 调用需带 `--token`。

### 3. 业务操作

```bash
# 查看个人信息
nazhi whoami --token "eyJ..."

# 列出所有任务
nazhi task list --token "eyJ..."

# 提交任务（从 JSON 字符串）
nazhi task submit --token "eyJ..." --payload '{"circleTaskId":1001,"name":"班会"}'

# 提交任务（从文件）
nazhi task submit --token "eyJ..." --payload @task.json

# 提交自我评价
nazhi self-eval submit --token "eyJ..." --comment "很好的学期"

# 查看评价状态
nazhi self-eval status --token "eyJ..."

# 上传图片（用于任务附件）
nazhi file upload -f ./photo.jpg
```

### 全局选项

所有命令支持：

| 标志 | 说明 |
|---|---|
| `-v, --verbose` | 详细日志输出到 stderr |
| `--quiet` | 静默模式 |
| `--output json` | 输出格式（默认 JSON） |

---

## 环境变量

所有 CLI 命令都支持通过环境变量注入凭据和配置，**命令行标志始终优先于环境变量**。

### 支持的变量

| 变量 | 作用 | 适用命令 |
|------|------|----------|
| `NAZHI_USERNAME` | 学号 | `login`、`school` |
| `NAZHI_PASSWORD` | 密码 | `login` |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval` |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login`、`school` |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval` |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 |

### 三种使用方式

**方式 1：临时环境变量**

```bash
export NAZHI_USERNAME="学号"
export NAZHI_PASSWORD="密码"
nazhi login                              # 不需要再传 -u/-p
nazhi task list                          # 先登录获取 token，或 export NAZHI_TOKEN=...
```

**方式 2：.env 文件（推荐用于本地开发）**

```bash
cp .env.example .env
# 编辑 .env 填入真实凭据
make test-integration                    # 集成测试自动读取 .env
```

`.env` 已在 `.gitignore` 中，不会被提交。CI 中使用 GitHub Secrets 注入即可。

**方式 3：CI 注入**

```yaml
# .github/workflows/ci.yml
- name: 集成测试
  env:
    NAZHI_USERNAME: ${{ secrets.NAZHI_USERNAME }}
    NAZHI_PASSWORD: ${{ secrets.NAZHI_PASSWORD }}
  run: go test -tags=integration -v ./test/integration/...
```

### 优先级

```
命令行标志 > 环境变量 > SDK 默认值
```

示例：

```bash
# 环境变量提供学号，命令行覆盖密码
NAZHI_USERNAME=学号 nazhi login -p 其它密码
```

---

## 命令参考

```
nazhi
├── login                          SSO 登录（全自动 OCR）
│   ├── -u/--username       必填   学号（NAZHI_USERNAME）
│   ├── -p/--password       必填   密码（NAZHI_PASSWORD）
│   ├── --sso-base          选填   SSO 根地址（NAZHI_SSO_BASE，默认 https://www.nazhisoft.com）
│   └── --timeout           选填   HTTP 超时秒数（NAZHI_TIMEOUT，默认 15）
│
├── school                          查询学校 ID（不需登录）
│   └── -u/--username       必填   学号（NAZHI_USERNAME）
│
├── session
│   └── activate                    激活业务 Session
│       └── --token          必填   X-Auth-Token（NAZHI_TOKEN）
│
├── whoami                          获取当前用户信息
│   └── --token            必填   （NAZHI_TOKEN）
│
├── task
│   ├── list                        列出全维度任务
│   │   └── --token        必填
│   └── submit                      提交任务
│       ├── --token        必填
│       └── --payload      必填     JSON 字符串或 @file.json
│
├── self-eval
│   ├── submit                      提交自我评价
│   │   ├── --token        必填
│   │   └── --comment      必填     支持 stdin: -
│   └── status                      查询评价状态
│       └── --token        必填
│
└── file
    └── upload                      上传图片
        └── -f/--file       必填   本地图片路径
```

### 输出格式

成功时输出 JSON 到 stdout：

```json
{ "code": 1, "msg": "成功", "data": [...] }
```

失败时输出 JSON 到 stderr 并退出码 1：

```json
{ "error": true, "message": "具体错误信息" }
```

可通过 `--quiet` 屏蔽所有 stderr 输出，便于脚本管道处理。

---

## 跨平台支持

| 平台 | 架构 | 状态 | 备注 |
|---|---|---|---|
| **Windows** | amd64 | ✅ | 主力测试平台 |
| **Windows** | arm64 | ✅ | Windows on ARM |
| **Linux** | amd64 | ✅ | 服务器主流 |
| **Linux** | arm64 | ✅ | ARM 服务器 / Raspberry Pi |
| **macOS** | arm64 | ✅ | Apple Silicon |
| macOS | x86_64 | ❌ | Microsoft 已停发 onnxruntime |

### OCR 原生库分发

每个平台携带对应 onnxruntime 库（C 引擎），通过 Go build tag 隔离嵌入：

```
internal/ocr/
├── onnx_win_amd64.go   //go:build windows && amd64
├── onnx_win_arm64.go   //go:build windows && arm64
├── onnx_lin_amd64.go   //go:build linux && amd64
├── onnx_lin_arm64.go   //go:build linux && arm64
└── onnx_mac_arm64.go   //go:build darwin && arm64
```

编译时只嵌入当前平台那份（约 15-37 MB），所以 Windows amd64 二进制不会带 macOS 的 dylib。

### OCR 进程级单例

`internal/ocr.GetDefault()` 进程共享一个 OCR 引擎：

- 多个 `client.New()` 共享同一 `*OCR` 实例
- 模型只解压一次（约 14 MB → 临时目录）
- 内部 `sync.Mutex` 保证并发安全
- 99 次重试机制（同一图片）提高识别准确率

### CI 矩阵

`onnxruntime_go` 在 Linux/macOS 强制 CGO，无法从其他 OS 交叉编译。CI 每个平台用 native runner：

- `ubuntu-latest` / `ubuntu-22.04-arm64` 编译 Linux（CGO=1）
- `macos-latest` 编译 macOS（CGO=1）
- `windows-latest` / `windows-11-arm` 编译 Windows（CGO=0）

---

## 作为 Go SDK 使用

### 快速上手

```go
import (
    "context"
    "log"
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)

func main() {
    // 创建客户端（OCR 默认启用、进程级单例）
    c := client.New(
        client.WithSSOBase("https://www.nazhisoft.com"),
        client.WithTimeout(15 * time.Second),
    )

    // 1. 登录（学号密码从配置/环境变量读取，不要硬编码）
    resp, err := c.Login(context.Background(), types.LoginRequest{
        Username: os.Getenv("NAZHI_USERNAME"),
        Password: os.Getenv("NAZHI_PASSWORD"),
    })
    if err != nil {
        log.Fatal(err)
    }
    token := resp.Token

    // 2. 激活 Session
    if _, err := c.ActivateSession(context.Background(), token); err != nil {
        log.Fatal(err)
    }

    // 3. 业务操作
    tasks, err := c.FetchTasks(context.Background(), token)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("共 %d 个任务", len(tasks))

    // 提交任务
    result, err := c.SubmitTask(context.Background(), token, types.TaskSubmitPayload{
        CircleTaskID: 1001,
        Name:         "班会",
        // ... 其他 28 个字段
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("提交结果: code=%d", result.Code)

    // 自我评价
    if err := c.SubmitSelfEvaluation(context.Background(), token, "很好的学期"); err != nil {
        log.Fatal(err)
    }

    // 上传图片
    imageID, err := c.UploadFile(context.Background(), "./photo.jpg")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("图片 ID: %d", imageID)
}
```

### 进阶选项

```go
// 自定义 HTTP 客户端
c := client.New(
    client.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:    100,
            IdleConnTimeout: 60 * time.Second,
        },
    }),
    client.WithLogger(slog.Default()),
)

// 修改 SSO 根地址（用于开发/测试环境）
c := client.New(
    client.WithSSOBase("http://localhost:8080"),
)
```

### 错误处理

SDK 通过 `errors.Is` 判断错误类型：

```go
import "errors"

_, err := c.Login(ctx, req)
switch {
case errors.Is(err, client.ErrLoginRejected):
    // 学号/密码错误
case errors.Is(err, client.ErrTokenExpired):
    // Token 过期，需重新登录
case errors.Is(err, client.ErrNetwork):
    // 网络问题（超时/断连）
}
```

完整错误列表见 `pkg/client/errors.go`。

### 线程安全

`Client` 实例是线程安全的：

- 独立 cookie jar（每个 Client 隔离）
- 独立 HTTP 连接池
- 共享进程级 OCR 引擎（`ocr.GetDefault()`）

可以在多个 goroutine 中并发调用同一 Client。

---

## 架构总览

```
nazhi-cli
├── cmd/nazhi/          ← CLI 层（cobra 命令）
│   ├── login.go
│   ├── school.go
│   ├── session.go
│   ├── task_*.go
│   ├── self_eval_*.go
│   ├── file_upload.go
│   ├── env.go          ← 环境变量加载
│   └── output.go       ← 统一 JSON 输出 + 错误处理
│
├── pkg/                ← 公开 SDK
│   ├── client/         ← Client + Option 模式
│   │   ├── auth.go           SSO 登录
│   │   ├── session.go        Session 激活
│   │   ├── task.go           任务 CRUD
│   │   ├── self_eval.go      自我评价
│   │   ├── user.go           用户信息
│   │   ├── file.go           文件上传
│   │   ├── client.go         Client 结构体 + Option
│   │   ├── request.go        HTTP 客户端封装
│   │   └── errors.go         哨兵错误
│   └── types/          ← 请求/响应类型
│
├── test/integration/   ← 真实环境集成测试（-tags=integration）
│   └── integration_test.go
│
└── internal/           ← 内部包（本仓库专用）
    ├── ocr/            ← ddddocr + onnxruntime 封装
    │   ├── ocr.go           OCR 服务（单例 + 平台分发）
    │   ├── onnx_*.go        build tag 隔离的原生库 embed
    │   └── models/          模型 + 字符集 + 5 平台 onnxruntime
    └── version/        ← 版本号
```

详细架构说明见 [CLAUDE.md](./CLAUDE.md)。

---

## 开发

### 常用命令

```bash
# 构建（当前平台）
make build

# 测试（含 race 检测）
make test               # 静默
make test-verbose       # 详细输出

# 集成测试（需要真实 SSO 凭据，存放在 .env 中）
make test-integration

# 代码质量
make lint               # golangci-lint
make vet                # go vet
make fmt                # gofmt

# 跨平台构建
make build-linux        # 交叉编译 Linux amd64
make build-darwin       # 交叉编译 macOS arm64
make build-windows      # 交叉编译 Windows amd64
make release            # 全平台发布

# 清理
make clean              # 清理 bin/ 等
```

### 项目要求

- Go 1.26+
- Windows / Linux / macOS

### 测试

```bash
# 全量单测
go test -race -count=1 ./...

# 仅 SDK 测试
go test -race -count=1 ./pkg/client/...

# 详细输出
go test -race -count=1 -v ./...

# 真实环境集成测试（需要 NAZHI_USERNAME/NAZHI_PASSWORD）
go test -race -count=1 -tags=integration -v ./test/integration/...
```

### 贡献

欢迎 PR！流程见 [CONTRIBUTING.md](./CONTRIBUTING.md)。提交前请确保：

- `make test` 通过
- `make lint` 通过
- 提交信息遵循 Conventional Commits
- **不要** 在代码、注释、文档中提交真实凭据（用占位符或 .env）

---

## 常见问题

### Q: Windows 上被杀毒软件误报？

A: 内嵌的 `onnxruntime.dll` 是 Microsoft 官方二进制，部分杀软会误报。这是 go-ddddocr + onnxruntime 生态的通用问题。建议在白名单中添加本程序，或在企业内网环境使用。

### Q: macOS x86_64 何时支持？

A: Microsoft onnxruntime v1.25.0 已停止发布 macOS x86_64 版本（Apple 全面转向 Silicon）。本项目不打算支持。如需 Intel Mac 请使用 v1.20.x 的 onnxruntime（需自行 fork OCR 库）。

### Q: 能否完全离线运行（不联网）？

A: ✅ 可以。OCR 模型在编译时通过 `//go:embed` 嵌入二进制，运行时仅访问 `www.nazhisoft.com`（SSO 服务器）。

### Q: 登录失败提示"验证码校验失败"？

A: OCR 对同一张验证码最多重试 99 次（神经网络本身有随机性）。如持续失败：
- 检查账号密码是否正确
- 重试 2-3 次（验证码会刷新）
- 可能是服务端限制，稍等几分钟

### Q: 如何在 CI 中调用 nazhi？

A: 直接下载 release 二进制到 runner，调用即可：

```yaml
- name: Login
  env:
    NAZHI_USERNAME: ${{ secrets.NAZHI_USERNAME }}
    NAZHI_PASSWORD: ${{ secrets.NAZHI_PASSWORD }}
  run: |
    ./nazhi login > token.json
    TOKEN=$(jq -r .token token.json)
    echo "TOKEN=$TOKEN" >> $GITHUB_ENV
- name: Fetch tasks
  env:
    NAZHI_TOKEN: ${{ env.TOKEN }}
  run: ./nazhi task list
```

### Q: SDK 是否能作为库被外部 Go 项目 import？

A: ✅ 可以。`pkg/client` 和 `pkg/types` 是公开包：

```go
import (
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)
```

`internal/ocr` 是内部包，外部项目无法导入，但不影响 SDK 使用（OCR 通过 `GetDefault()` 自动初始化）。

### Q: Linux ARM64 (Raspberry Pi) 能否运行？

A: ✅ 可以，已在 CI 验证。注意 onnxruntime ARM64 二进制需要 ARMv8-A 架构（树莓派 4+）。

### Q: 如何调试 HTTP 请求？

A: 使用 `--verbose` 标志可看到详细日志：

```bash
nazhi login -v
```

输出包含请求方法、URL、状态码、耗时等。

### Q: 集成测试和单元测试的区别？

A:

- **单元测试** (`make test`)：用 `httptest.Server` mock SSO/业务服务器，验证 SDK 内部逻辑、并发、错误处理等。CI 必跑。
- **集成测试** (`make test-integration`)：连真实 SSO/业务服务器，验证真实环境下的登录、API 调用。需要 `NAZHI_USERNAME` / `NAZHI_PASSWORD` 环境变量（未设置时自动 `t.Skip`）。本地手动跑，CI 可选。

---

## 协议

[MIT License](./LICENSE) — 详见 LICENSE 文件。

---

*Built with ❤️ for 纳智综合评价系统自动化*
