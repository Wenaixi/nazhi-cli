# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![Version](https://img.shields.io/badge/version-1.3.0-blue)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

一站式命令行工具 + Go SDK，用于纳智综合评价系统的自动化操作。提供 SSO 登录（OCR 自动识别验证码）、Session 激活、任务管理、自我评价、文件上传等完整功能。所有 CLI 命令统一 envelope 输出，便于脚本解析。

## 仓库一览

| 入口 | 说明 |
|---|---|
| [快速开始](#快速开始) | 5 分钟登录并跑通业务 |
| [文档中心](docs/README.md) | docs 总索引 |
| [CLI 参考](docs/cli/README.md) | 命令 / 环境变量 / envelope / 短示例 |
| [SDK 参考](docs/sdk/README.md) | 总览 + 按功能分册（请求/响应/用法） |
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

> **注意：`make build` 已知坑**：`build-*` target 都未带 `-tags=ddddocr`，本机构建出的二进制 `c.ocr=nil`，
> `nazhi login` 会立即返回 `ErrOCRNotConfigured`。本地想跑通登录必须显式带 tag：
>
> ```bash
> go build -tags=ddddocr -o bin/nazhi.exe ./cmd/nazhi
> ```
>
> 只有 CI 的 `build` / `release` job 显式带了 `-tags=ddddocr`。详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 快速开始

```bash
# 1. 登录拿 token（envelope 输出，提取 .data.token）
export NAZHI_USERNAME=学号
export NAZHI_PASSWORD=密码
TOKEN=$(nazhi login | jq -r .data.token)
export NAZHI_TOKEN=$TOKEN

# 2. 激活业务 Session（HAR 对齐 4 步，登录后必做一次）
nazhi session activate

# 3. 业务操作
nazhi whoami
nazhi task list
nazhi task submit --payload @task.json
nazhi self-eval submit --comment "很好的学期"
nazhi self-eval submit --payload '{"bxqhzr":"会做人目标","bxqbx":"本学期表现","bxqys":"优势"}'
nazhi self-eval status

# 4. 上传图片（独立服务，不需要 token）
nazhi file upload -f ./photo.jpg

# 5. 下载附件（按 ID 拿到 task submitted 里的图片）
nazhi task submitted | jq -r '.data.records[].imgList[].attachment_id' | \
  xargs -I {} nazhi file download --id {} --output ./img_{}.jpg
```

更详细的环境变量与命令说明见 [CLI 参考](docs/cli/README.md)。

> 自我评价支持纯文本和结构化 `--payload` 两种提交方式；结构化表单会由 SDK 双层包装为 `studentComment`。毕业评价的查询/提交仅提供 Go SDK 方法，当前 CLI 不提供对应命令。荣誉删除使用 GET，并通过 `id` 查询参数传递记录 ID。详见 [SDK 自评](docs/sdk/self-eval.md) 与 [SDK 荣誉](docs/sdk/honor.md)。

> 写实 `task submit` / `task edit` 的 `--payload` 可直接使用真实前端表单 JSON；`hours`、`level`、`checkResult`、`playRole` 兼容 number/string，CLI 在 `cmd/nazhi` 输入边界归一后再交给 SDK；同时兼容 `circleTaskId` → `taskId`、`pictureList` → `imageIDs` 两个前端字段别名，规范字段优先。任务元数据和图片由 SDK 自动补齐；Hours 是否可省略取决于任务元数据，空地址和空等级不会被 SDK 自动替换。详见 [SDK 任务文档](docs/sdk/task.md)。

## 命令概览

```
nazhi
├── login                       SSO 登录（全自动 OCR）
├── session
│   └── activate                 激活业务 Session（HAR 4 步）
├── whoami                      获取当前用户信息（含 schoolId）
├── task
│   ├── list                     列出全维度任务
│   ├── submit                   提交任务（支持 @payload.json）
│   ├── submitted                获取班级已提交写实记录（含同班同学姓名/学号）
│   ├── done                     同 task submitted（v1.0.0 新增别名）
│   ├── teacher                  获取教师代写的写实记录
│   ├── public                   获取公示的全部写实记录
│   ├── withdrawn                获取被撤回的写实记录
│   └── edit                     修改已提交的写实记录
├── self-eval
│   ├── submit                   提交自我评价
│   └── status                   查询评价状态 + 教师评语
├── circle
│   ├── delete                   删除写实记录
│   ├── comment                  添加写实评论
│   └── like                     点赞/取消点赞
├── honor
│   ├── types                    获取荣誉类型列表
│   ├── list                     获取已申报荣誉记录
│   ├── add                      申报荣誉（支持 @payload.json）
│   └── delete                   删除荣誉记录
├── typical-case
│   ├── submit                   提交典型案例
│   ├── list                     获取典型案例（可 --status 筛审核状态）
│   ├── update                   更新典型案例
│   └── delete                   删除典型案例
├── user
│   ├── info                     查看个人信息（whoami 别名）
│   └── update                   更新个人信息
├── file
│   ├── upload                   上传图片（独立公共服务，不接受 --token）
│   └── download                 下载附件图片（不接受 --token）
├── version                     显示版本信息
└── completion                  生成 shell 自动补全脚本
```

完整参数与 JSON 输出字段见 [CLI 参考](docs/cli/README.md)。

> **v1.0.0 移除**：`nazhi school` 命令已删除，学校 ID 现从 `nazhi whoami` 返回的 `data.schoolId` 字段获取。

## envelope 输出格式

v1.0.0 起所有 CLI 输出统一包装在 envelope 内：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "id": 12345,
    "name": "张三",
    "studentNumber": "G123456789012345678",
    "schoolId": 11000001,
    "schoolName": "纳智高中",
    "gradeId": 12,
    "gradeName": "高一",
    "classId": 88,
    "className": "八班"
  }
}
```

字段说明：

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | `success` / `partial` / `error` |
| `code` | int | 业务 code（1=成功）或 HTTP 状态码（200/401/500 等） |
| `message` | string | 错误或提示消息，成功时为空 |
| `data` | any | 业务载荷（object / array / scalar） |

退出码三分：

| 退出码 | 含义 |
|--------|------|
| 0 | 成功（status=success） |
| 1 | 业务错误（code != 1）或 partial 状态 |
| 2 | 网络/服务端错误（HTTP 4xx/5xx） |
| 3 | 参数错误（CLI flag 解析失败） |

jq 解析示例：

```bash
# 提取业务数据
nazhi whoami | jq '.data.name'

# 提取 token
TOKEN=$(nazhi login | jq -r '.data.token')

# 判断成功
if [ $(nazhi task list | jq -r '.status') = "success" ]; then
  echo "OK"
fi
```

> **设计说明** `file upload` 子命令**不接受 `--token`** 是有意设计：上传服务器 `doc.nazhisoft.com` 是独立公共服务，
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
| `NAZHI_USERNAME` | 学号 | `login` | — |
| `NAZHI_PASSWORD` | 密码 | `login` | — |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval`、`honor` | — |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login` | `https://www.nazhisoft.com` |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval`、`honor` | `http://139.159.205.146:8280` |
| `NAZHI_UPLOAD_URL` | 文件上传服务器 | `file upload` | `http://doc.nazhisoft.com` |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 | `15`（`file upload` 是 `30`） |

详见 [CLI 参考 · 环境变量](docs/cli/README.md#环境变量速查)。

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

### Push 前必跑（CI 6 步）

详见 [CONTRIBUTING.md](CONTRIBUTING.md) 的「push 前必跑」章节——6 个独立 gate（mod tidy / lint / vet / gofmt / test / integration build）必须全绿。

### 贡献

欢迎 PR！流程见 [CONTRIBUTING.md](CONTRIBUTING.md)。提交规范遵循 Conventional Commits，
中文描述也可以接受。PR 提交前必跑上面 6 步。

## 安全

历史事故：早期版本曾有真实学号密码泄露到 git 历史（已用 `git-filter-repo`
彻底清除并 force push）。如果您使用过早期版本，请在 SSO 平台修改密码。

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
