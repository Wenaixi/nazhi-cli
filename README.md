# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![Version](https://img.shields.io/badge/version-1.3.0-blue)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

一站式命令行工具 + Go SDK，用于纳智综合评价系统的自动化操作。提供 SSO 登录（调用视觉识别器自动处理验证码）、Session 激活、任务管理、自我评价、文件上传等完整功能。所有 CLI 命令统一 envelope 输出，便于脚本解析。

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

- **视觉模型验证码识别** — 登录验证码通过 `WithCustomOCR` 注入 Nazhi-auto 同款硅基流动 Qwen3-Omni，支持多图重试
- **纯 Go 构建** — SDK 不内置本地验证码识别器、模型或原生运行库，无 CGO、无额外模型文件
- **统一配置** — CLI 正式读取 `NAZHI_SILICONFLOW_API_KEY`，兼容 `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY`
- **HAR 验证 4 步 Session 激活** — `pkg/client/session.go` 的 `sessionManager` 状态机 + DCL fast-path + 同 token backoff 缓存
- **完整错误链** — 15 个哨兵错误（`ErrNetwork` / `ErrRateLimited` / `ErrRetryable` 等），`errors.Is` 精确分支
- **Cookie + Header 双重 Token 注入** — 业务服务器要求 `X-Auth-Token` 双形态存在，SDK 一次性处理
- **并发安全** — 每个 `*Client` 独立 cookie jar，atomic.Pointer 保护 baseURL 预解析热路径无锁
- **HAR 驱动测试 + PII 守卫** — 真实抓包做 fixture，自带 SHA-256 哈希反 PII 泄露自反性陷阱

## 安装

### 预编译二进制（推荐）

从 [Releases](https://github.com/Wenaixi/nazhi-cli/releases) 下载对应平台的二进制：

| 平台 | 架构 | 文件 |
|---|---|---|
| Windows | amd64 / arm64 | `nazhi-windows-amd64.exe` / `nazhi-windows-arm64.exe` |
| Linux | amd64 / arm64 | `nazhi-linux-amd64` / `nazhi-linux-arm64` |
| macOS | arm64 (Apple Silicon) | `nazhi-darwin-arm64` |

> 各平台发布包均为纯 Go 二进制；验证码视觉模型通过运行时 API 配置，不随二进制打包。

### `go install`

```bash
go install github.com/Wenaixi/nazhi-cli/cmd/nazhi@latest
```

### 从源码构建

```bash
git clone https://github.com/Wenaixi/nazhi-cli.git
cd nazhi-cli
make build           # 当前平台纯 Go 构建
make release         # 全平台纯 Go 构建（CI 等价）
```

> **登录需要视觉模型密钥**：运行 `nazhi login` 前设置 `NAZHI_SILICONFLOW_API_KEY`。
> 该变量对应 Nazhi-auto 的正式配置；CLI 仍兼容 `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY`。
> 未配置视觉识别器时不会退回本地实现，而是返回 `ErrOCRNotConfigured`。

## 快速开始

```bash
# 1. 配置 Nazhi-auto 同款视觉模型并登录
export NAZHI_SILICONFLOW_API_KEY=sk-...
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

> 自我评价支持纯文本和结构化 `--payload` 两种提交方式；结构化表单会由 SDK 双层包装为 `studentComment`。毕业评价通过 `self-eval grad-status/grad-submit` 透传 SDK 查询与提交能力。荣誉下拉分为类型选项 `honor type-options`、通用等级 `honor level-options` 和按类型联动等级 `honor levels --type-id`；荣誉删除使用 GET，并通过 `id` 查询参数传递记录 ID。详见 [SDK 自评](docs/sdk/self-eval.md) 与 [SDK 荣誉](docs/sdk/honor.md)。

> 写实 `task submit` / `task edit` 的 `--payload` 可直接使用真实前端表单 JSON；`hours` 的 number/string 均兼容且可保留小数，`level`、`checkResult`、`playRole` 的未加引号 number 必须是有限整数，`1.0`/`1e0` 等会规范为标准十进制代码字符串，小数、非有限值和溢出值会被拒绝，string 按原值保留。`--payload -` 从 stdin 读取时上限为 16 MiB，超限会按参数错误处理，不会静默截断。CLI 在 `cmd/nazhi` 输入边界归一后再交给 SDK；同时兼容 `circleTaskId` → `taskId`、`pictureList` → `imageIDs` 两个前端字段别名，规范字段优先。任务元数据和图片由 SDK 自动补齐；Hours 是否可省略取决于任务元数据，空地址和空等级不会被 SDK 自动替换。详见 [SDK 任务文档](docs/sdk/task.md)。

## 命令概览

```
nazhi
├── login                       SSO 登录（视觉识别器自动处理验证码）
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
│   ├── edit                     修改已提交的写实记录
│   ├── dimensions              获取写实维度列表
│   └── circle-type              获取任务写实元数据
├── self-eval
│   ├── submit                   提交自我评价
│   ├── status                   查询评价状态 + 教师评语
│   ├── grad-status              查询毕业评价原始状态
│   └── grad-submit              提交毕业评价
├── circle
│   ├── delete                   删除写实记录
│   ├── comment                  添加写实评论
│   ├── like                     点赞/取消点赞
│   ├── types                    按维度获取写实类别
│   ├── tasks                    按类别获取写实任务
│   ├── images                   分页获取写实图片
│   └── dict                     按分类获取系统字典
├── honor
│   ├── types                    获取荣誉类型列表
│   ├── type-options             获取荣誉类型下拉（dataList）
│   ├── level-options             获取通用等级下拉（returnData）
│   ├── list                     获取已申报荣誉记录
│   ├── add                      申报荣誉（支持 @payload.json）
│   ├── update                   更新荣誉记录
│   ├── levels                   按类型获取联动等级
│   └── delete                   删除荣誉记录
├── typical-case
│   ├── submit                   提交典型案例
│   ├── list                     获取典型案例（可 --status 筛审核状态）
│   ├── update                   更新典型案例
│   ├── delete                   删除典型案例
│   └── delete-batch             批量删除典型案例
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

recognizer := newMyCaptchaRecognizer()
c, err := client.New(
    client.WithSSOBase("https://www.nazhisoft.com"),   // 可省，默认就是这个
    client.WithBaseURL("http://139.159.205.146:8280"), // 可省，默认就是这个
    client.WithTimeout(30 * time.Second),
    client.WithSessionBackoff(5 * time.Second), // 调 Session 激活失败冷却窗口
    client.WithCustomOCR(recognizer), // Login 必需
)
if err != nil { log.Fatalf("Client 初始化失败：%v", err) }
defer c.Close()

// 登录（调用视觉识别器自动处理验证码）
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
make release           # 全平台纯 Go 构建

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

- [硅基流动](https://siliconflow.cn/) — Qwen3-Omni 视觉模型服务
- [cobra](https://github.com/spf13/cobra) — CLI 框架

---

*Built for nazhi 综合评价系统自动化*
