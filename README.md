# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

一站式命令行工具 + Go SDK，用于纳智综合评价系统的自动化操作。提供 SSO 登录（OCR 自动识别验证码）、Session 激活、任务管理、自我评价、文件上传等完整功能。所有 CLI 命令输出 JSON，便于脚本解析。

## 仓库一览

| 入口 | 说明 |
|---|---|
| [快速开始](#快速开始) | 5 分钟登录并跑通业务 |
| [环境变量](docs/env-vars.md) | NAZHI_* 完整说明 |
| [CLI 参考](docs/cli/README.md) | 每个命令的 flag / 输出 / 示例 |
| [SDK 参考](docs/sdk/README.md) | Go SDK API（pkg/client + pkg/types + pkg/tokenparse） |
| [架构总览](docs/architecture.md) | 双层架构、关键决策 |
| [登录流程详解](docs/login-flow.md) | SSO 多图多试 + 4 步 Session 激活 |
| [跨平台 OCR](docs/cross-platform-ocr.md) | 5 平台 onnxruntime + Windows DLL 修复三轮演化 |
| [HAR 驱动测试](docs/har-testing.md) | 抓包驱动集成测试 + PII SHA-256 守卫 |
| [开发指南](#开发) | 构建、测试、贡献流程 |
| [CHANGELOG](CHANGELOG.md) | 全部版本变更日志 |
| [CLAUDE.md](CLAUDE.md) | 项目记忆库（git 忽略，AI 协作专用） |

## 特色

- **跨平台 OCR** — Windows / Linux / macOS × amd64 / arm64 共 5 个组合，onnxruntime 原生库 `//go:embed` 进二进制
- **开箱即用** — OCR 模型 + 字符集嵌入，零下载、零配置（默认 `-tags ddddocr` 构建）
- **可选 CGO-free 构建** — `go build` 不带 tag 时仅依赖纯 Go，外部 OCR 通过 `WithCustomOCR` 注入
- **HAR 验证 4 步 Session 激活** — `pkg/client/session.go` 的 `sessionManager` 状态机 + DCL fast-path + 同 token backoff 缓存
- **完整错误链** — 15 个哨兵错误（`ErrNetwork` / `ErrRateLimited` / `ErrRetryable` 等），`errors.Is` 精确分支
- **Cookie + Header 双重 Token 注入** — 业务服务器要求 `X-Auth-Token` 双形态存在，SDK 一次性处理
- **并发安全** — 每个 `*Client` 独立 cookie jar，atomic.Pointer 保护 baseURL 预解析热路径无锁
- **Windows OCR 自愈** — DLL 句柄未释放降级（不再污染 stderr）+ 启动时 best-effort 清扫历史 temp 目录
- **HAR 驱动测试 + PII 守卫** — 真实抓包做 fixture，自带 SHA-256 哈希反 PII 泄露自反性陷阱

## 安装

### 预编译二进制（推荐）

从 [Releases](https://github.com/Wenaixi/nazhi-cli/releases) 下载对应平台的二进制：

| 平台 | 架构 | 文件 |
|---|---|---|
| Windows | amd64 / arm64 | `nazhi-windows-amd64.exe` / `nazhi-windows-arm64.exe` |
| Linux | amd64 / arm64 | `nazhi-linux-amd64` / `nazhi-linux-arm64` |
| macOS | arm64 (Apple Silicon) | `nazhi-darwin-arm64` |

> macOS 仅 arm64（Microsoft 已停发 onnxruntime macOS x86_64）。

### `go install`

```bash
go install github.com/Wenaixi/nazhi-cli/cmd/nazhi@latest
```

### 从源码构建

```bash
git clone https://github.com/Wenaixi/nazhi-cli.git
cd nazhi-cli
make build           # 当前平台（**已知坑：不含 OCR**，见下）
make release         # 全平台（CI 等价，含 OCR + CGO）
```

> ⚠️ **`make build` 已知坑**：`build-*` target 都未带 `-tags=ddddocr`，本机构建出的二进制 `c.ocr=nil`，
> `nazhi login` 会立即返回 `ErrOCRNotConfigured`。本地想跑通登录必须显式带 tag：
>
> ```bash
> go build -tags=ddddocr -o bin/nazhi.exe ./cmd/nazhi
> ```
>
> 只有 CI 的 `build` / `release` job 显式带了 `-tags=ddddocr`。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 快速开始

```bash
# 1. 登录拿 token（输出 JSON 到 stdout）
export NAZHI_USERNAME=学号
export NAZHI_PASSWORD=密码
TOKEN=$(nazhi login | jq -r .token)
export NAZHI_TOKEN=$TOKEN

# 2. 激活业务 Session（HAR 对齐 4 步，登录后必做一次）
nazhi session activate

# 3. 业务操作
nazhi whoami
nazhi task list
nazhi task submit --payload @task.json
nazhi self-eval submit --comment "很好的学期"
nazhi self-eval status

# 4. 上传图片（独立服务，不需要 token）
nazhi file upload -f ./photo.jpg
```

更详细的环境变量配置见 [docs/env-vars.md](docs/env-vars.md)。

## 命令概览

```
nazhi
├── login                       SSO 登录（全自动 OCR）
├── school                      查询学校 ID（无需登录）
├── session
│   └── activate                 激活业务 Session（HAR 4 步）
├── whoami                      获取当前用户信息
├── task
│   ├── list                     列出全维度任务
│   └── submit                   提交任务（支持 @payload.json）
├── self-eval
│   ├── submit                   提交自我评价
│   └── status                   查询评价状态 + 教师评语
├── file
│   └── upload                   上传图片（独立公共服务，不接受 --token）
├── version                     显示版本信息
└── completion                  生成 shell 自动补全脚本
```

完整参数与 JSON 输出字段见 [CLI 参考](docs/cli/README.md)。

> 💡 `file upload` 子命令**不接受 `--token`** 是有意设计：上传服务器 `doc.nazhisoft.com` 是独立公共服务，
> SDK 内部使用独立 `http.Client`（无 cookie jar + 禁用重定向），不发送任何业务 token，
> 避免给公共服务发送业务域 token 触发风控。

## 作为 Go SDK 使用

```go
import (
    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
    "github.com/Wenaixi/nazhi-cli/pkg/tokenparse" // SSO token 解析（独立可用）
)

c, err := client.New(
    client.WithSSOBase("https://www.nazhisoft.com"),   // 可省，默认就是这个
    client.WithBaseURL("http://139.159.205.146:8280"), // 可省，默认就是这个
    client.WithTimeout(30 * time.Second),
    client.WithSessionBackoff(5 * time.Second), // 调 Session 激活失败冷却窗口
)
if err != nil { log.Fatalf("Client 初始化失败：%v", err) }
defer c.Close()

// 登录（含 OCR 自动识别）
resp, err := c.Login(ctx, types.LoginRequest{
    Username: os.Getenv("NAZHI_USERNAME"),
    Password: os.Getenv("NAZHI_PASSWORD"),
})
token := resp.Token

// 激活 Session（HAR 4 步）
if _, err := c.ActivateSession(ctx, token); err != nil { log.Fatal(err) }

// 业务操作
tasks, err := c.FetchTasks(ctx, token)
c.SubmitSelfEvaluation(ctx, token, "很好的学期")
```

完整 API、所有 Option、15 个哨兵错误、错误处理骨架见 [SDK 参考](docs/sdk/README.md)。

## 环境变量速查

所有 CLI 命令都支持环境变量 fallback，**命令行标志始终优先于环境变量**：

| 变量 | 作用 | 适用命令 | 默认值 |
|---|---|---|---|
| `NAZHI_USERNAME` | 学号 | `login`、`school` | — |
| `NAZHI_PASSWORD` | 密码 | `login` | — |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval` | — |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login`、`school` | `https://www.nazhisoft.com` |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval` | `http://139.159.205.146:8280` |
| `NAZHI_UPLOAD_URL` | 文件上传服务器 | `file upload` | `http://doc.nazhisoft.com` |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 | `15`（`file upload` 是 `30`） |

详见 [docs/env-vars.md](docs/env-vars.md)。

## 开发

### 常用命令

```bash
make build              # 当前平台（见上"已知坑"）
make release           # 全平台（含 OCR + CGO）

make test              # 单元测试（race）
make test-verbose      # 详细测试输出
make test-integration  # 真实环境（需要 .env）

make lint              # golangci-lint
make vet               # go vet（多个 build tag）
make fmt               # gofmt

make clean             # 清理构建产物
```

### 真实环境集成测试

需要 `NAZHI_USERNAME` / `NAZHI_PASSWORD` 环境变量（或 `.env` 文件）：

```bash
cp .env.example .env
# 编辑 .env 填入真实凭据（推荐 `vim -n .env`，密码不进 shell 历史）
make test-integration
```

`.env` 已在 `.gitignore` 中，不会被提交。详见 [SECURITY.md](SECURITY.md)。

### Push 前必跑（CI 6 步铁律）

详见 [CONTRIBUTING.md](CONTRIBUTING.md) 的「push 前必跑」章节——6 个独立 gate（mod tidy / lint / vet / gofmt / test / integration build）必须全绿。

### 贡献

欢迎 PR！流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。提交规范遵循 Conventional Commits，
中文描述也可以接受。**PR 提交前必跑**上面 6 步铁律。

## 安全

⚠️ **重要历史事故**：早期版本曾有真实学号密码泄露到 git 历史（v0.2.0 之前，已用 `git-filter-repo` 彻底清除并 force push）。
如果您使用过早期版本，**务必在 SSO 平台修改密码**。

仓库测试与文档**绝不**包含真实 PII——`test/integration/har_pii_redacted_test.go` 用 SHA-256 哈希单向防御
PII 自反性陷阱（详见 [SECURITY.md](SECURITY.md)）。

`CLAUDE.md` 含架构细节和本地凭据，已 `.gitignore` 第 49 行隔离。

## 协议

[MIT License](LICENSE)

## 致谢

- [ddddocr](https://github.com/sml2h3/ddddocr) — OCR 引擎
- [Microsoft onnxruntime](https://github.com/microsoft/onnxruntime) — 模型推理
- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [yangbin1322/go-ddddocr](https://github.com/yangbin1322/go-ddddocr) — Go 绑定

---

*Built for nazhi 综合评价系统自动化*
