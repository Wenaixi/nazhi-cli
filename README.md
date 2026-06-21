# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

一站式命令行工具，用于纳智综合评价系统的自动化操作。提供完整的 **SSO 登录**、**任务管理**、**自我评价**、**文件上传** 等功能。

✨ **特色：内置 OCR 验证码自动识别** — 模型已内嵌至二进制，无需下载，无需配置，开箱即用。

---

## 安装

### 预编译二进制

从 [Releases](https://github.com/Wenaixi/nazhi-cli/releases) 下载对应平台的最新版本即可使用。

### go install

```bash
go install github.com/Wenaixi/nazhi-cli/cmd/nazhi@latest
```

## 快速开始

```bash
# 全自动登录（OCR 自动识别验证码，默认启用）
nazhi login -u S1234567890 -p TestPass123

# 查询学校 ID
nazhi school -u S1234567890

# 激活业务 Session
nazhi session activate --token eyJhbGciOiJIUzI1NiJ9.xxx

# 查看用户信息
nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx

# 列出任务
nazhi task list --token eyJhbGciOiJIUzI1NiJ9.xxx

# 提交任务
nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload '{"circleTaskId":1001}'
nazhi task submit --token eyJhbGciOiJIUzI1NiJ9.xxx --payload @task.json

# 自我评价
nazhi self-eval submit --token eyJhbGciOiJIUzI1NiJ9.xxx --comment "很好的学期"

# 文件上传
nazhi file upload -f ./photo.jpg
```

### OCR 验证码模式

验证码由内置 OCR 全自动识别（模型已通过 `go:embed` 内嵌在二进制中）。首次调用时自动解压到临时目录，无需网络下载。整个过程完全自动化，无需人工干预。

OCR 引擎基于 ddddocr（ONNX Runtime），对同一张图片最多重试 99 次以提高准确率。

## 命令参考

```
nazhi
├── login                          SSO 登录（全自动 OCR）
│   ├── -u/--username       必填   学号
│   ├── -p/--password       必填   密码
│   ├── --sso-base          选填   SSO 根地址（默认 https://www.nazhisoft.com）
│   └── --timeout           选填   HTTP 超时秒数（默认 15）
├── school                          查询学校 ID
│   └── -u/--username       必填   学号
├── session
│   └── activate                    激活业务 Session
│       └── --token          必填   X-Auth-Token
├── whoami                          获取用户信息
│   └── --token            必填
├── task
│   ├── list                        任务列表
│   │   └── --token        必填
│   └── submit                      提交任务
│       ├── --token        必填
│       └── --payload      必填     JSON 或 @file.json
├── self-eval
│   ├── submit                      提交自我评价
│   │   ├── --token        必填
│   │   └── --comment      必填     从 stdin 读取: -
│   └── status                      查询评价状态
│       └── --token        必填
└── file
    └── upload                      上传图片
        └── -f/--file       必填     本地图片路径
```

## 作为 Go SDK 使用

```go
import (
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)

// 创建客户端（OCR 验证码识别器默认启用，模型已内嵌）
c := client.New(
    client.WithSSOBase("https://www.nazhisoft.com"),
    client.WithTimeout(15*time.Second),
)

// 全自动登录（OCR 自动识别验证码）
resp, err := c.Login(ctx, types.LoginRequest{
    Username: "S1234567890",
    Password: "TestPass123",
    // 无需 Captcha 字段，OCR 自动处理
})
if err != nil { panic(err) }
fmt.Println("Token:", resp.Token)

// 获取任务列表
tasks, err := c.FetchTasks(ctx, resp.Token)

// 提交任务
err = c.SubmitTask(ctx, resp.Token, taskPayload)

// 提交自我评价
err = c.SubmitSelfEvaluation(ctx, resp.Token, "很好的学期")
```

### 自定义 HTTP 客户端

```go
c := client.New(
    client.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:    10,
            IdleConnTimeout: 30 * time.Second,
        },
    }),
    client.WithLogger(slog.Default()),
)
```

## API / SDK 包结构

```
pkg/
├── client/        # Go SDK：Client + Option 模式
│   ├── auth.go    #   登录 / 验证码 / Session
│   ├── task.go    #   任务 CRUD
│   ├── selfeval   #   自我评价
│   ├── file.go    #   文件上传
│   └── errors.go  #   错误定义
└── types/         # 请求/响应类型定义
```

## 开发

```bash
# 构建
make build

# 运行测试（含 race 检测）
make test

# 代码检查
make vet

# 跨平台发布
make release

# 清理
make clean
```

### 构建产物

| 目标 | 命令 | 输出 |
|---|---|---|
| Windows | `make build` | `bin/nazhi.exe` |
| Linux | `make build-linux` | `bin/nazhi-linux-amd64` |
| macOS | `make build-darwin` | `bin/nazhi-darwin-amd64` |
| 全平台 | `make release` | `bin/` 下所有 |

## 协议

[MIT License](LICENSE) — 详见 LICENSE 文件。

---

*Built with ❤️ for 纳智综合评价系统自动化*
