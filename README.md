# nazhi-cli

**纳智综合评价系统 自动化 CLI + Go SDK**

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Wenaixi/nazhi-cli)](https://github.com/Wenaixi/nazhi-cli/releases)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/Wenaixi/nazhi-cli/ci.yml?branch=main)](https://github.com/Wenaixi/nazhi-cli/actions)

一站式命令行工具 + Go SDK，面向纳智综合评价系统的全流程自动化：SSO 登录（视觉识别器自动处理验证码）、Session 激活、写实记录全生命周期管理（查询 / 提交 / 编辑 / 删除 / 评论 / 点赞）、任务与元数据、荣誉申报、典型案例、自我评价与毕业评价、用户信息维护、文件上传下载。所有 CLI 命令输出统一 JSON envelope，便于脚本解析与自动化编排。

## 仓库一览

| 入口 | 说明 |
|---|---|
| [快速开始](#快速开始) | 5 分钟登录并跑通业务 |
| [命令概览](#命令概览) | 全部子命令速查树 |
| [作为 Go SDK 使用](#作为-go-sdk-使用) | 在 Go 项目中集成 |
| [源码指引](docs/README.md) | 功能 ↔ Go 源码 ↔ 前端源码 对照表 |
| [开发指南](#开发) | 构建、测试、贡献流程 |
| [CHANGELOG](CHANGELOG.md) | 全部版本变更日志 |

## 特色

- **视觉模型验证码识别** — 登录验证码通过 `WithCustomOCR` 注入 Nazhi-auto 同款硅基流动 Qwen3-Omni，支持多图重试
- **纯 Go 构建** — SDK 不内置本地验证码识别器、模型或原生运行库，无 CGO、无额外模型文件
- **统一配置** — CLI 正式读取 `NAZHI_SILICONFLOW_API_KEY`，兼容 `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY`
- **HAR 验证 4 步 Session 激活** — `pkg/client/session.go` 的 `sessionManager` 状态机 + DCL fast-path + 同 token backoff 缓存
- **完整错误链** — 16 个哨兵错误（`ErrNetwork` / `ErrRateLimited` / `ErrRetryable` 等），`errors.Is` 精确分支
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

要求 Go 1.26+（以仓库 `go.mod` 为准）。

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
nazhi task submitted | jq -r '.data[]?.imgList[]?.attachment_id // .data.records[]?.imgList[]?.attachment_id' | \
  xargs -I {} nazhi file download --id {} --output ./img_{}.jpg
```

更详细的环境变量见下方[环境变量速查](#环境变量速查)；各命令的参数与输出字段直接跑 `nazhi <命令> --help` 或读 `cmd/nazhi/` 对应源文件。

> 自我评价支持纯文本与结构化 `--payload` 两种提交；毕业评价走 `self-eval grad-status / grad-submit`。
> 荣誉下拉分三类：类型选项 `honor type-options`、通用等级 `honor level-options`、按类型联动 `honor levels --type-id`。

<details>
<summary><strong>写实 payload 兼容性细节（点开查看）</strong></summary>

- `--payload` 可直接使用真实前端表单 JSON，字段以纳智前端 `managementRightBottom.vue` 表单为准（仓库外本地镜像，不随本仓库分发）
- 类型宽容：`hours` 接受 number/string 且可保留小数；`level` / `checkResult` / `playRole` 的裸 number 必须是有限整数，string 原值保留
- 字段别名兼容：`circleTaskId` → `taskId`、`pictureList` → `imageIDs`（规范字段优先）
- `--payload -` 从 stdin 读取，上限 16 MiB，超限按参数错误处理不静默截断
- 任务元数据与图片由 SDK 自动补齐；空地址 / 空等级**不会**被自动替换（对齐前端行为）

</details>

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
│   ├── submitted                获取我发布的写实记录（type=3，仅本人；全班公示用 public）
│   ├── done                     同 task submitted（别名）
│   ├── teacher                  获取教师代写的写实记录
│   ├── public                   获取公示的全部写实记录
│   ├── withdrawn                获取被撤回的写实记录
│   ├── edit                     修改已提交的写实记录
│   ├── preview                  预览提交/编辑的最终 JSON payload（不调写接口）
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

完整参数与 JSON 输出字段跑 `nazhi <命令> --help` 查看，或读 `cmd/nazhi/` 对应源文件。

> `nazhi school` 命令已移除，学校 ID 从 `nazhi whoami` 返回的 `data.schoolId` 字段获取。

## envelope 输出格式

所有 CLI 输出统一包装在 envelope 内：

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
| `code` | int | HTTP 风格状态码：成功为 `200`（空数据成功为 `204`，如写操作成功），失败为 4xx/5xx |
| `message` | string | 错误或提示消息，成功时为空 |
| `data` | any | 业务载荷（object / array / scalar） |

> **双层 code 对照**：CLI 信封的 `envelope.code` 表示成功（200 或空数据成功 204）；平台原始业务响应
> （`pkg/types.UnifiedResponse.code`）的成功值是 `1`。两层同名不同层——
> 脚本判成功请统一用 `.status == "success"`（兼容 200 与 204 两种成功形态），**不要**只判 `.code == 200`，也**不要**沿用业务层 `jq .code==1` 的习惯。

退出码三分：

| 退出码 | 含义 |
|--------|------|
| 0 | 成功（status=success） |
| 1 | partial 部分成功，或业务错误（HTTP 4xx 非 400） |
| 2 | 网络/服务端错误（HTTP 5xx） |
| 3 | 参数错误（HTTP 400，含 payload 解析失败） |

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
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
	"github.com/Wenaixi/nazhi-cli/pkg/tokenparse" // SSO token 解析（可独立使用）
)

// CaptchaRecognizer 由调用方实现（AI 视觉模型 / 远程服务 / 测试 mock）
recognizer := newMyCaptchaRecognizer()
c, err := client.New(
	client.WithSSOBase("https://www.nazhisoft.com"),   // 可省，默认即此
	client.WithBaseURL("http://139.159.205.146:8280"), // 可省，默认即此
	client.WithTimeout(30*time.Second),
	client.WithSessionBackoff(5*time.Second), // Session 激活失败冷却窗口
	client.WithCustomOCR(recognizer),         // Login 必需，未注入返回 ErrOCRNotConfigured
)
if err != nil {
	log.Fatalf("Client 初始化失败：%v", err)
}
defer c.Close()

ctx := context.Background()

// 登录（识别器自动处理验证码；凭据错返回 ErrLoginRejected）
resp, err := c.Login(ctx, types.LoginRequest{
	Username: os.Getenv("NAZHI_USERNAME"),
	Password: os.Getenv("NAZHI_PASSWORD"),
})
if err != nil {
	log.Fatal(err)
}
token := resp.Token

// 激活业务 Session（HAR 对齐 4 步；冷却窗口内重复调用返回 ErrSessionBackoff）
if _, err := c.ActivateSession(ctx, token); err != nil {
	log.Fatal(err)
}

// 业务操作：全部方法先自动激活 Session，token 过期自动重试激活
tasks, err := c.FetchTasks(ctx, token)
if err != nil {
	// 哨兵错误精确分支示例
	if errors.Is(err, client.ErrBusinessRejected) {
		log.Fatal("服务端拒绝：session 可能已过期")
	}
	log.Fatal(err)
}
_ = tasks

err = c.SubmitSelfEvaluation(ctx, token, "很好的学期")
if err != nil {
	log.Fatal(err)
}
```

完整 API 与所有 Option 见各源码文件的包注释（`pkg/client`、`pkg/types`），功能↔文件对照见[源码指引](docs/README.md)。

## 环境变量速查

所有 CLI 命令都支持环境变量 fallback，**命令行标志始终优先于环境变量**：

| 变量 | 作用 | 适用命令 | 默认值 |
|---|---|---|---|
| `NAZHI_SILICONFLOW_API_KEY` | 硅基流动 Qwen3-Omni 视觉模型密钥，**登录必填**（兼容别名见下） | `login` | — |
| `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY` | 上者的兼容别名 | `login` | — |
| `NAZHI_USERNAME` | 学号 | `login` | — |
| `NAZHI_PASSWORD` | 密码 | `login` | — |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval`、`honor` | — |
| `NAZHI_LOG_LEVEL` | 日志级别 debug/info/warn/error | 全局 | `warn` |
| `NAZHI_LOG_FORMAT` | 日志格式 text/json | 全局 | `text` |
| `NAZHI_LOG_FILE` | 日志落盘路径（quiet 仍写入） | 全局 | — |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login` | `https://www.nazhisoft.com` |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval`、`honor` | `http://139.159.205.146:8280` |
| `NAZHI_UPLOAD_URL` | 文件上传服务器 | `file upload` | `http://doc.nazhisoft.com` |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 | `15`（`file upload` 是 `30`） |

各命令还支持 `--help` 查看全部 flag 与环境变量说明。

## 开发

### 常用命令

```bash
make build              # 当前平台纯 Go 构建
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

**历史凭据事件披露**：早期开发阶段曾有真实学号密码进入 git 历史，已用 `git-filter-repo`
彻底清除对象库并重写。若您拉取过受影响时期的仓库副本（worktree / 本地克隆），请同步更新；
使用过该凭据的用户应在纳智 SSO 平台修改密码。

**PII 防护承诺**：仓库测试与文档绝不包含真实姓名、学号、密码原文。
`test/integration/har_pii_redacted_test.go` 以 SHA-256 摘要单向比对，防御守卫文件自身成为泄露源的自反性陷阱。

漏洞报告流程与完整安全策略见 [SECURITY.md](SECURITY.md)。

## 协议

[MIT License](LICENSE)

## 致谢

- [硅基流动](https://siliconflow.cn/) — Qwen3-Omni 视觉模型服务
- [cobra](https://github.com/spf13/cobra) — CLI 框架

---

*Built for nazhi 综合评价系统自动化*