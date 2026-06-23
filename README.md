# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK** v0.2.2

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

一站式命令行工具 + Go SDK，用于纳智综合评价系统的自动化操作。提供 **SSO 登录**、**任务管理**、**自我评价**、**文件上传** 等完整功能。

## 特色

- 全自动 OCR 验证码 — 模型已嵌入二进制，无需下载、无需配置，开箱即用
- 跨平台支持 — Windows / Linux / macOS（5 个平台 × 架构组合），单二进制运行
- 进程级 OCR 单例 — 多实例共享引擎，避免重复解压模型
- CLI + SDK 双形态 — 脚本可调用，集成方导入 Go 包
- 完整测试覆盖 — 单元测试 + race 检测 + 真实环境集成测试 + HAR 驱动测试
- HAR 对齐 — 4 步 Session 激活、双重 Token 注入，全部逆向抓包验证

## 📚 文档

- [快速开始](#快速开始)
- [安装](#安装)
- [环境变量](#环境变量)
- [命令参考](docs/cli/README.md)
- [SDK 参考](docs/sdk/README.md)
- [架构总览](docs/architecture.md)
- [登录流程详解](docs/login-flow.md)
- [跨平台 OCR](docs/cross-platform-ocr.md)
- [HAR 驱动测试](docs/har-testing.md)
- [开发指南](#开发)
- [CHANGELOG](CHANGELOG.md)

## 安装

### 预编译二进制（推荐）

从 [Releases](https://github.com/Wenaixi/nazhi-cli/releases) 下载：

| 平台 | 架构 | 文件 |
|---|---|---|
| Windows | amd64 / arm64 | `nazhi-windows-amd64.exe` |
| Linux | amd64 / arm64 | `nazhi-linux-amd64` |
| macOS | arm64 (Apple Silicon) | `nazhi-darwin-arm64` |

> macOS 仅支持 arm64（Microsoft 已停发 onnxruntime macOS x86_64）

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

## 快速开始

### 1. 登录获取 Token

```bash
# 方式 1：命令行参数
nazhi login -u 学号 -p 密码

# 方式 2：环境变量（推荐用于 CI）
export NAZHI_USERNAME=学号
export NAZHI_PASSWORD=密码
nazhi login

# 方式 3：.env 文件
cp .env.example .env  # 编辑填入真实凭据
nazhi login
```

### 2. 完整流程

```bash
# 登录拿 token（输出 JSON 到 stdout）
TOKEN=$(nazhi login | jq -r .token)
export NAZHI_TOKEN=$TOKEN

# 激活业务 Session（HAR 对齐 4 步）
nazhi session activate

# 业务操作
nazhi whoami
nazhi task list
nazhi task submit --payload @task.json
nazhi self-eval submit --comment "很好的学期"
nazhi self-eval status

# 上传图片（独立，不需要 token）
nazhi file upload -f ./photo.jpg
```

## 环境变量

所有 CLI 命令都支持通过环境变量注入凭据和配置：

| 变量 | 作用 | 适用命令 |
|------|------|----------|
| `NAZHI_USERNAME` | 学号 | `login`、`school` |
| `NAZHI_PASSWORD` | 密码 | `login` |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval` |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login`、`school` |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval` |
| `NAZHI_UPLOAD_URL` | 文件上传服务器 | `file upload` |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 |

**优先级**：命令行标志 > 环境变量 > SDK 默认值

详见 [env-vars.md](docs/env-vars.md)

## 命令概览

```
nazhi
├── login                # SSO 登录（全自动 OCR）
├── school               # 查询学校 ID
├── session
│   └── activate         # 激活业务 Session
├── whoami               # 获取用户信息
├── task
│   ├── list             # 任务列表
│   └── submit           # 提交任务
├── self-eval
│   ├── submit           # 提交自我评价
│   └── status           # 查询评价状态
└── file
    └── upload           # 上传图片
├── version              # 显示版本信息
└── completion           # 生成 shell 自动补全脚本
```

详见 [CLI 参考](docs/cli/README.md)

## 作为 Go SDK 使用

```go
import (
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)

c := client.New(
    client.WithSSOBase("https://www.nazhisoft.com"),
    client.WithTimeout(30 * time.Second),
)

// 登录
resp, _ := c.Login(ctx, types.LoginRequest{
    Username: os.Getenv("NAZHI_USERNAME"),
    Password: os.Getenv("NAZHI_PASSWORD"),
})
token := resp.Token

// 激活 Session
c.ActivateSession(ctx, token)

// 业务操作
tasks, _ := c.FetchTasks(ctx, token)
c.SubmitSelfEvaluation(ctx, token, "很好的学期")
```

详见 [SDK 参考](docs/sdk/README.md)

## 开发

### 常用命令

```bash
# 构建
make build              # 当前平台
make release           # 全平台

# 测试
make test              # 单元测试（race）
make test-verbose      # 详细输出
make test-integration  # 真实环境（需要 .env）

# 代码质量
make lint              # golangci-lint
make vet               # go vet
make fmt               # gofmt

# 清理
make clean
```

### 真实环境集成测试

需要 `NAZHI_USERNAME` / `NAZHI_PASSWORD` 环境变量（或 .env 文件）：

```bash
cp .env.example .env
# 编辑 .env 填入真实凭据
make test-integration
```

### 贡献

欢迎 PR！流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。提交前请确保：

- `make test` 通过
- `make lint` 通过
- 提交信息遵循 Conventional Commits
- **不要** 在代码、注释、文档中提交真实凭据

## 安全

⚠️ **重要**：历史版本中曾发生过学号密码泄露事故（已在 0.2.0 通过 `git-filter-repo` 修复）。如果您使用过早期版本，**务必在 SSO 平台修改密码**。

详见 [SECURITY.md](SECURITY.md)

## 协议

[MIT License](LICENSE)

## 致谢

- [ddddocr](https://github.com/sml2h3/ddddocr) — OCR 引擎
- [Microsoft onnxruntime](https://github.com/microsoft/onnxruntime) — 模型推理
- [cobra](https://github.com/spf13/cobra) — CLI 框架

---

*Built for nazhi 综合评价系统自动化*
