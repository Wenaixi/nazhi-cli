# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

**nazhi-cli** — 纳智综合评价自动化 CLI + Go SDK。

一站式命令行工具 + Go SDK，用于纳智综合评价系统的自动化操作。提供 SSO 登录、任务管理、自我评价、文件上传等功能。

## 根目录文件

| 路径 | 类型 | 说明 |
|---|---|---|
| `.gitattributes` | 配置文件 | Git 属性（行尾符、二进制识别等） |
| `.github/` | 目录 | GitHub Actions CI/CD 工作流 |
| `.gitignore` | 配置文件 | Git 忽略规则（构建产物、临时文件、IDE 配置、CLAUDE.md 等） |
| `.golangci.yml` | 配置文件 | golangci-lint 检查规则（启用 errcheck/govet/unused，豁免 cmd/ 工具代码和 _test.go） |
| `CHANGELOG.md` | 文档 | 版本变更日志（面向用户） |
| `CLAUDE.md` | 文档 | 项目记忆（git 忽略，AI 协作专用，含敏感架构信息） |
| `CONTRIBUTING.md` | 文档 | 贡献指南（PR 流程、代码规范） |
| `LICENSE` | 文档 | MIT 许可证 |
| `Makefile` | 脚本 | 构建脚本（`build`/`test`/`lint`/`release` 等 target） |
| `README.md` | 文档 | 项目主文档（用户入口、徽章、命令速查） |
| `SECURITY.md` | 文档 | 安全策略（漏洞上报方式、支持版本） |
| `bin/` | 目录 | `go build` 产物目录（**git 忽略**） |
| `cmd/` | 目录 | CLI 入口（cobra 命令的薄壳层） |
| `go.mod` | Go 模块 | Go 模块定义（依赖 + Go 1.26.1 版本） |
| `go.sum` | Go 模块 | 依赖校验和 |
| `internal/` | 目录 | 内部包（仅本仓库可用，含 OCR 引擎、版本号等） |
| `pkg/` | 目录 | 公开 Go SDK（`pkg/client` + `pkg/types`） |
| `tmp_onnx_download/` | 目录 | OCR 原生库下载临时目录（**git 忽略**） |

> 不在仓库的运行时文件：`.omc/`（OMC 多智能体编排状态）、`.idea/` `.vscode/`（IDE 配置）、`captcha.jpg` `*.png` `*.log`（调试临时）—— 全部 `.gitignore` 已处理。

## 构建与测试命令

```bash
# 编译 CLI 到 bin/
make build
go build -o bin/nazhi.exe ./cmd/nazhi

# 运行全量测试（race 检测）
make test
go test -count=1 -race ./...

# 详细测试输出
make test-verbose
go test -count=1 -race -v ./...

# 集成测试（需要 integration build tag）
make test-integration

# 代码质量检查
make lint        # golangci-lint（配置文件 .golangci.yml）
make vet         # go vet
make fmt         # gofmt 格式化

# 跨平台发布构建
make release

# 清理构建产物
make clean
```

## CI/CD

`.github/workflows/ci.yml` — GitHub Actions 工作流，三个 Job 串联：

| Job | 触发 | 内容 |
|-----|------|------|
| **check** | push/PR/tag | golangci-lint → go vet → `go test -race ./...` |
| **build** | check 通过 | 6 目标同时构建：linux amd64/arm64, darwin amd64/arm64, windows amd64/arm64 |
| **release** | 仅 tag v* | 收集所有产物 → 创建 GitHub Release 附件 |

产物命名：`nazhi-<version>-<os>-<arch>`（Windows 加 `.exe`）。

### golangci-lint 配置

`.golangci.yml` 规则要点：
- 启用 `errcheck`、`govet`、`unused`
- `cmd/` 目录下的调试/工具代码整体豁免 `errcheck`
- 测试文件 `_test.go` 豁免 `errcheck` 和 `unused`
- 显式排除 `io.Copy`、`json.Unmarshal`、`Encode`、`Decode`、`Scanf` 等通常不检查的场景

## 代码架构

### 双层架构

```
cmd/nazhi/          # CLI 层 — cobra 命令（薄壳，只做参数解析和 JSON 输出）
└── pkg/client/     # SDK 层 — 核心业务逻辑
    └── pkg/types/  # 共享类型定义
```

CLI 所有命令与 SDK 方法一一对应：

| CLI 命令 | SDK 方法 | 文件 |
|----------|----------|------|
| `nazhi login` | `c.Login()` | `auth.go` |
| `nazhi school` | `c.GetSchoolID()` | `auth.go` |
| `nazhi session activate` | `c.ActivateSession()` | `session.go` |
| `nazhi whoami` | `c.GetMyInfo()` | `user.go` |
| `nazhi task list` | `c.FetchTasks()` | `task.go` |
| `nazhi task submit` | `c.SubmitTask()` | `task.go` |
| `nazhi self-eval submit` | `c.SubmitSelfEvaluation()` | `self_eval.go` |
| `nazhi self-eval status` | `c.QuerySelfEvaluation()` | `self_eval.go` |
| `nazhi file upload` | `c.UploadFile()` | `file.go` |
| `nazhi version` | - | `version.go` |
| `nazhi completion` | - | `completion.go` |

### 目录结构

```
├── cmd/nazhi/               # CLI 入口
│   ├── main.go              # cobra root command，注册所有子命令
│   ├── login.go             # nazhi login — SSO 登录（全自动 OCR）
│   ├── school.go            # nazhi school — 查询学校 ID
│   ├── session.go           # nazhi session activate — 激活业务 Session
│   ├── whoami.go            # nazhi whoami — 获取用户信息
│   ├── task_list.go         # nazhi task list — 全维度任务列表
│   ├── task_submit.go       # nazhi task submit — 提交任务（支持 @file.json）
│   ├── self_eval_submit.go  # nazhi self-eval submit — 提交自我评价
│   ├── self_eval_status.go  # nazhi self-eval status — 查询评价状态
│   ├── file_upload.go       # nazhi file upload — 上传图片
│   ├── version.go           # nazhi version — 显示版本信息
│   ├── completion.go        # nazhi completion — shell 自动补全
│   ├── parents.go           # 父命令定义（task, self-eval, file）
│   └── output.go            # 统一 JSON 输出 + 错误处理
├── pkg/client/               # Go SDK 核心
│   ├── client.go             # Client 结构体 + Option 模式构造函数
│   ├── request.go            # HTTP 客户端 + httpDo/rawDoWithResp（候选 #2 私有化）
│   ├── errors.go             # 哨兵错误（ErrLoginRejected, ErrNetwork 等）
│   ├── auth.go               # 认证流程（InitSession → GetSchoolID → Captcha → Login）
│   ├── session.go            # ActivateSession（HAR 验证的两步初始化）
│   ├── session_manager.go    # sessionManager 字段封装（候选 #5）
│   ├── task.go               # FetchTasks（多维度遍历聚合）、SubmitTask、GetDimensions
│   ├── self_eval.go          # SubmitSelfEvaluation、QuerySelfEvaluation
│   ├── user.go               # GetMyInfo
│   ├── file.go               # UploadFile（multipart 上传）
│   ├── parallel.go           # ❌ 未实现（候选 #6/#7，尚未创建）
│   ├── error_category.go     # ❌ 未实现（候选 #8，尚未创建）
│   ├── cookie_sync.go        # syncCookieToken + buildLoginResponse 辅助
│   └── client_test.go        # 单元测试（httptest mock server）
├── pkg/tokenparse/           # SSO 登录 token 解析（候选 #4，新建包）
│   ├── tokenparse.go         # ExtractFromLocation / ExtractFromReturnData
│   └── tokenparse_test.go    # 289 行测试
├── pkg/types/
│   ├── types.go              # 领域类型（LoginRequest/Response, Task, UserInfo 等）
│   ├── response.go           # UnifiedResponse + 泛型解码辅助方法 + DecodeUnified（候选 #3）
│   └── deref.go              # 泛型 DerefOr[T] 安全解引用（候选 #4）
├── internal/version/         # CLI 版本号（当前 0.3.5）
├── internal/ocr/             # 验证码 OCR
│   ├── ocr.go                # ddddocr 封装，go:embed 嵌入模型，跨平台 build tag 隔离
│   ├── ocr_close_test.go     # Close 资源清理测试
│   ├── onnx_win_amd64.go     # 嵌入 Windows amd64 onnxruntime.dll
│   ├── onnx_win_arm64.go     # 嵌入 Windows arm64 onnxruntime.dll
│   ├── onnx_lin_amd64.go     # 嵌入 Linux amd64 libonnxruntime.so
│   ├── onnx_lin_arm64.go     # 嵌入 Linux arm64 libonnxruntime.so
│   ├── onnx_mac_arm64.go     # 嵌入 macOS arm64 libonnxruntime.dylib
│   └── models/               # 嵌入的 ONNX 模型 + 字符集 + 各平台原生库
├── .github/workflows/ci.yml  # CI 工作流
├── .golangci.yml             # golangci-lint 配置
├── .gitignore
├── Makefile
├── go.mod                    # Go 1.26.1, spf13/cobra, go-ddddocr
├── CLAUDE.md                 # 项目记忆（git 忽略）
└── README.md
```

## 架构关键决策

### 1. Option 模式构造 Client

`pkg/client/client.go` — 每个 Client 拥有独立的 HTTP cookie jar，天然并发安全。

```go
c := client.New(
    client.WithSSOBase("https://www.nazhisoft.com"),
    client.WithTimeout(15*time.Second),
)
// OCR 验证码识别器默认启用（模型已内嵌在二进制中）
```

### 2. 自定义 Redirect Handler

`pkg/client/request.go:31` — 不自动跟随 302 重定向，因为 SSO 登录成功的 token 在 Location 头中。

```go
CheckRedirect: func(req *http.Request, via []*http.Request) error {
    return http.ErrUseLastResponse
},
```

### 3. 统一响应体解析（泛型辅助）

`pkg/types/response.go` — 目标平台所有 API 返回统一 JSON 结构。使用 Go 泛型提供类型安全的解码：

- `DecodeReturnData[T]()` — 解析 returnData 字段
- `DecodeDataList[T]()` — 解析 dataList 字段为切片
- `DecodeDataMap[T]()` — 解析 dataMap 字段
- `CheckCode()` — 检查 code 是否为 1（成功）

### 4. OCR 跨平台 + 进程级单例

`internal/ocr/ocr.go` — 5 平台原生库 build tag 隔离嵌入（不含 macOS x86_64：Microsoft onnxruntime v1.25.0 已停止发布 macOS Intel 版本）：

```
internal/ocr/
├── onnx_win_amd64.go   //go:build windows && amd64
├── onnx_win_arm64.go   //go:build windows && arm64
├── onnx_lin_amd64.go   //go:build linux && amd64
├── onnx_lin_arm64.go   //go:build linux && arm64
└── onnx_mac_arm64.go   //go:build darwin && arm64
```

每个文件都暴露 `var OnnxRuntimeDLL []byte`，Go 编译时按 (GOOS, GOARCH) 只取一份嵌入二进制。

**OCR 并发**：`ocr.Pool` 基于 `sync.Pool` 实现实例复用。默认 `NewPool(0)` 懒加载 1 个 `*OCR` 实例，需 `sync.Mutex` 串行化识别调用。`WithOCRConcurrency(n)` 预热 n 个独立 ONNX session 实例，支持 n 路真并发（每个约 50MB 内存）。

**为什么必须解压到磁盘**：`go-ddddocr` → `onnxruntime_go` v1.25.0 强制要磁盘路径（C 运行时 `dlopen` / `LoadLibrary` 不支持内存模块）。

**单例机制**：
- `ocr.GetDefault()` 进程级单例，所有 Client 共享同一引擎
- 模型只解压一次，多个 Client 不再产生多个临时目录
- `sync.Mutex` 保护 `Classification()` 并发安全
- 内部 `sync.Once` 仍负责惰性初始化

**文件命名规则**（按平台）：

| GOOS | 文件名 |
|---|---|
| windows | `onnxruntime.dll` |
| linux | `libonnxruntime.so` |
| darwin | `libonnxruntime.dylib` |

`platformLibName()` 根据 `runtime.GOOS` 动态返回解压时的目标文件名。

**下载源**：从 Microsoft onnxruntime v1.25.0 releases 下载（与 `onnxruntime_go` v1.25.0 ABI 对齐）。

### 5. HAR 验证的 Session 激活

`pkg/client/session.go` — 业务 API session 需要先 GET `/`（首页）再 GET `/api/studentInfo/getMenu`，否则后续接口返回空数据。这个顺序通过抓包（HAR）验证。

### 6. Login 全流程（多图多试 OCR）

`pkg/client/auth.go` — SSO 登录内部流程：

```
InitSession (GET /uiStudentLogin/login → 建立 JSESSIONID Cookie)
  → GetSchoolID (POST ...getSchoolIdByStudentNumber)
  → OCR 多图多试识别验证码（1 张图 OCR 1 次 × 最多 99 张图 = 99 次总尝试上限）
  → 预校验验证码 (POST .../validateCaptcha)
  → 正式登录 (POST .../validate) → 302 → 从 Location 头提取 JWT token
```

**OCR 重试策略（v0.2.1+）**：多图多试策略。单张图片 OCR 1 次（ddddocr 对同一张图是确定性的，同图重试无意义），失败则换新图，最多换 99 张。总尝试次数上限 = 1 x 99 = 99 次。
- 旧策略（v0.2.0）：单图 3 次 x 33 张图 = 99 次
- 当前策略（v0.2.1+）：单图 1 次 x 99 张图 = 99 次总尝试上限，换图才是真正有效
- 常量定义在 `auth.go`：`maxOCRAttemptsPerImage = 1`、`maxOCRImagesTotal = 99`

`LoginRequest` 中无 `Captcha` 字段，无需调用方关心验证码。

测试时通过 `WithCustomOCR()` 注入 `captchaRecognizer` 接口 mock（`ocr_retry_test.go` 提供 7 个测试覆盖单图成功/换图重试/全失败/空结果/图片拉取失败等场景）。

### 7. 测试策略

`pkg/client/client_test.go` — 使用 `httptest.Server` 创建 mock 服务端，验证：
- HTTP 方法、路径、请求头
- 请求体字段
- 调用顺序（通过 callStep 计数器）
- 错误路径（密码错误、验证码错误）
- 并发隔离（多个 Client 实例的 cookie jar 独立性）

### 8. ~~Debug 工具目录~~

~~`cmd/debuglogin/`、`cmd/reallogin/` 是早期调试/原型验证工具，**不是生产代码**。`golangci-lint` 对这些目录做了豁免（通过 `.golangci.yml` 的 `exclude-rules`）。<br>已删除的目录：~~`cmd/getcaptcha/`~~、~~`cmd/ocrtest/`~~（SSO 去除 FetchCaptcha 公共方法后不再需要）。~~<br>已删除的目录：~~`cmd/debuglogin/`~~、~~`cmd/reallogin/`~~（raw HTTP 调试工具，正式 SDK 上线后不再需要）。~~

> 所有 debug/原型工具均已清理，只保留 `cmd/nazhi/`（生产 CLI 入口）。

### 敏感凭据记录

> ⚠️ 以下凭据在早期版本中已被泄露到 git 历史，已通过 `git-filter-repo` 彻底清除。

| 字段 | 值 | 说明 |
|------|-----|------|
| 学号 | `G350181200912110035`（git-filter-repo 已清除） | 纳智 SSO 登录账号，已被历史提交泄露 |
| 密码 | `689050`（git-filter-repo 已清除） | 对应学号的密码，已被历史提交泄露 |

**影响范围**：所有分支（`main`、`feat/har-alignment`、`feat/har-alignment-v2`）和标签 `v0.1.0`。

**补救措施**：
- `git-filter-repo` 将历史中所有出现替换为占位值
- 测试代码改用 `TEST2025001` / `TestPass123`
- 文档和 CLI example 改用 `学号` / `S1234567890`
- 远程仓库已 force push 覆盖
- **用户务必在 SSO 平台修改密码**，因为凭据在清除前已在公开仓库暴露过

## ⚠️ 必读：PII 守卫自反性 bug（2026-06-28 事故教训）

### 事故回顾

`test/integration/har_pii_redacted_test.go` 原本设计为「PII 守卫」——扫描所有测试文件和 HAR fixture 防止真实 PII 泄露。但**它自己反而成了 PII 泄露源**：

```go
// ❌ 反面教材（v0.3.5 前的实现）
const (
    realStudentNumber    = "G" + "350181200912110035"  // ← 真实学号明文！
    realIDCard           = "3" + "50181200912110035"   // ← 真实身份证明文！
    realStudentName      = "高" + "博文"               // ← 真实姓名明文！
    realNamePinyin       = "ga" + "obowen"
    realNameInitials     = "g" + "bw"
    realStudentID        = "38" + "7020"
    realUserID           = "32" + "7053"
    realStudyNumber      = "2508" + "010404"
)
```

作者用「字符串拼接」绕过守卫自身的 AST 自检，但这只是视觉欺骗——**字符串字面量在源码和编译产物中都是明文可见的**。

### 修复方案

**用预计算的 SHA-256 十六进制摘要替代明文 PII**：

```go
// ✅ 正确实现（v0.3.5+）
piiHexMap := map[string]string{
    "931577eabd71afd0475218f0b676da9712c5f03150f5fc3035109d9dcdd00896": "学号",
    "8182e345670a08de6afcc00ed0688a180a812f3c544a28fb5330c9af3b7c8974": "身份证号",
    "537d18920afded93ad219b5e59370f40bbc59c07a00b21484795c8d4ff849743": "真实姓名",
    // ... 只存 hex 摘要，不可逆
}

// 扫描时对候选字符串计算 SHA-256，再查表
h := sha256.Sum256([]byte(s))
if desc, found := piiHexMap[h]; found { /* 命中 PII */ }
```

**安全保证**：SHA-256 是单向函数，源码中只存 64 字符 hex 摘要，**任何人看到 hex 也无法反推出原始 PII**。hex 摘要不构成泄露。

### git 历史清理

主仓库和 24 个 worktree 都需要处理。完整流程：

1. **主仓库历史**：`git filter-repo --replace-text` 配合拼接模式匹配清理所有拆开片段
2. **commit message**：`git filter-repo --replace-message` 清理提到账号的 commit 描述
3. **worktree 副本**：filter-repo 不动 worktree 文件，需手动 `cp` 主仓库清洁版到每个 `.claude/worktrees/*/test/integration/har_pii_redacted_test.go`

### 反思

- **「PII 守卫」是最容易写错的安全代码**——它的目的就是检查 PII，所以必须包含 PII 示例；但包含又成了泄露。这是经典「自己吃自己狗粮」陷阱
- **字符串拼接绕过 AST 自检 = 假安全感**——只防住了「自己看自己」，防不住「别人看你」
- **正确做法是「用 PII 的不可逆表示（哈希）」而非「PII 本身」**——哈希表的大小比明文小，安全性更高
- **worktree 是 filter-repo 的盲区**——filter-repo 只重写主仓库的对象库，worktree 的 working tree 文件是独立副本，需手动同步

## 重要约定

- CLI 输出统一使用 JSON 格式（`printJSON` 到 stdout，`printError` 到 stderr`）
- 通过 `-v/--verbose` 控制 stderr 日志输出
- SDK 方法命名：动词 + 名词（FetchTasks, SubmitTask, QuerySelfEvaluation）
- 模块划分：一个文件一个职责领域（auth, task, self_eval, file, user, session）
- TaskSubmitPayload 结构体 29 字段透传，不裁剪不处理
- `@file.json` 语法支持从文件读取 JSON 请求体
- 版本号维护在 `internal/version/version.go` 中，CI 和 Makefile 从此文件读取
- 不得提交 `CLAUDE.md` 到 git 仓库（已加入 `.gitignore`）

### 输出通道例外（避免误判 bug）

CLI 严格遵循「stdout = JSON 输出 / stderr = JSON 错误 + verbose 日志」双通道契约，但以下路径直写 stderr 是**有意的设计**，不是绕过：

| 路径 | 位置 | 意图 |
|------|------|------|
| `printPrompt` | `cmd/nazhi/output.go:81` | stdin 交互提示（如 self-eval submit 的「请输入评价: 」），**不**受 verbose 守卫，但受 `isTerminalStdin()` + `quiet` 守卫 |
| `c.logger.Warn` 资源警告 | `pkg/client/*.go`（如 `pool.Close` 失败、`http.CloseIdleConnections` 异常） | 直接走用户注入的 slog handler，不走 printError（保持 SDK 纯净，不引入 cmd 依赖） |
| backoff 冷却提示 | `cmd/nazhi/session.go` | 捕获 `ErrSessionBackoff` 时输出 `{"status":"cooldown","message":"..."}` 让脚本感知等待 |

**为什么需要文档化**：维护者看到直写 stderr 第一反应是「绕过 printError，应重构」。但 `printPrompt` 走 printError 会污染 stderr 错误流（混进 JSON envelope），走 printVerbose 用户没加 `-v` 就看不到提示——只有直写 stderr 才能同时满足「用户可见 + 不污染错误流」两个约束。

## ⚠️ 必读：pre-push 验证铁律（v0.3.5 事故教训）

### 事故回顾

2026-06-27 v0.3.5 release 第一次 push（commit `d9e40b5`）时，CI check job 报 **golangci-lint FAIL**：

```
Error: pkg/client/response_decode.go:44:6: func `tryDecodeFallbackWithPost` is unused (unused)
```

根因：`tryDecodeFallback` 修复时设计了 `tryDecodeFallback` + `tryDecodeFallbackWithPost` 两个变体，
但所有调用方只用了无 post 版本，post 版本从未被调用。**我（主代理）只跑了 `go test` 就 push**，
没跑 golangci-lint。CI 因此挂了 1 分钟，浪费了主人数分钟往返。

修复：commit `c5992c3` 删除 `tryDecodeFallbackWithPost` 函数，CI 重新绿。

### 铁律：push 前必跑完整 CI 步骤

**绝对不能**只跑 `go test` 就 push。CI workflow 包含 5 个独立检查，
每个都可能单独 fail：

| 步骤 | 命令 | 失败时表现 |
|------|------|------------|
| 1. `go mod tidy` 整洁 | `go mod tidy && git diff --exit-code go.mod go.sum` | 多余/缺失 dep 阻塞 |
| 2. `golangci-lint` | `"$(go env GOPATH)/bin/golangci-lint" run --timeout=5m ./...` | unused / shadow / gosimple 等 |
| 3. `go vet` | `go vet ./...` + `go vet -tags=integration ./test/integration/...` | 类型/调用错误 |
| 4. `gofmt` | `gofmt -l .` 非空 = 失败 | 格式不规范 |
| 5. `go test -race` | `go test -count=1 -race -v -timeout=15m ./pkg/...` | 逻辑错误 |

**一键验证脚本**（push 前必跑）：

```bash
# 1. go mod tidy 整洁
go mod tidy && git diff --exit-code go.mod go.sum

# 2. golangci-lint（若本机未装：go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8）
"$(go env GOPATH)/bin/golangci-lint" run --timeout=5m ./...

# 3. go vet（两个 build tag 都要跑）
go vet ./...
go vet -tags=integration ./test/integration/...

# 4. gofmt
if [ "$(gofmt -l . | wc -l)" -gt 0 ]; then echo "FAIL: $(gofmt -l .)"; exit 1; fi

# 5. 单元测试（race 检测）
go test -count=1 -race -v -timeout=15m ./pkg/...

# 6. 集成测试编译验证
go test -tags=integration -run=^$ ./test/integration/...
```

**全部绿**才能 push。任何一步红 → 立即修，不要等 CI 报。

### 适用场景

- ✅ 任何 commit 后准备 push
- ✅ fixer agent 完成一组修复、merge 之前
- ✅ 全部子任务合入后
- ✅ Release tag 推送前

### 反思

**只跑 `go test` ≠ 「CI 会过」**。CI 包含 5 个独立 gate，每个都有不同侧重：
- test 验证逻辑
- lint 验证风格 + dead code
- vet 验证类型系统约束
- gofmt 验证代码格式
- mod tidy 验证依赖一致性

下次再偷懒只跑 test，我（主代理）就在 commit message 里给自己写检讨 🐱。

## OCR 可选构建设计（v0.3.5）

### 设计动机

`internal/ocr` 包依赖 `go-ddddocr` → `onnxruntime_go` v1.25.0，**强制 CGO**。
- 服务端 Go 程序：必须 CGO_ENABLED=1，能 link onnxruntime
- CGO-free 消费者（如 Nazhi-auto 嵌入式 SDK）：**无法** link onnxruntime

如果默认 build 强制 ddddocr → CGO-free 消费者编译失败。
如果默认 build 不含 OCR → 普通用户 CLI 调 Login() 返回 `ErrOCRNotConfigured`，体验崩坏。

**解法：build tag 二选一**（与 Go 生态惯例对齐）：

| 构建方式 | 命令 | OCR 行为 | 适用场景 |
|---------|------|---------|----------|
| **含 OCR**（默认 release）| `go build -tags=ddddocr` | 内嵌 ddddocr + onnxruntime，开箱即用 | CLI / 服务端 Go 程序 |
| **CGO-free 纯 Go** | `go build`（无 tag）| `c.ocr = nil`，需 `WithCustomOCR` 注入 | 嵌入式 / 不允许 CGO 的消费者 |

### 实施细节

```
pkg/client/
├── client.go                      # 公共 Client 抽象
├── client_ocr_disabled.go         # //go:build !ddddocr
│   ├── defaultOCR() → nil
│   └── WithOCRConcurrency(n) → warn（占位）
└── client_ocr_enabled.go          # //go:build ddddocr
    ├── defaultOCR() → ocr.NewPool(0)
    └── WithOCRConcurrency(n) → ocr.NewPool(n)
```

`Login()` 入口检查 `c.ocr == nil` → 返回 `ErrOCRNotConfigured` + 提示注入 `WithCustomOCR`。

### ⚠️ CI 必须显式加 `-tags=ddddocr`（教训）

**v0.3.5 release 事故 #2**：第一次 push 的 CI workflow 编译命令 `go build ./cmd/nazhi` 缺 `-tags=ddddocr`，
意味着发布的 5 平台二进制是 `!ddddocr` 构建 → `c.ocr = nil` → **用户用 release 二进制调 `nazhi login` 立即失败**！

主银质问「OCR可选不内内置？」才暴露这个 bug。修复：
- `ci.yml` build 步骤加 `-tags=ddddocr`（release 二进制含 OCR）
- `ci.yml` test 步骤跑两遍：默认 + `-tags=ddddocr`（验证两套 build 都健康）
- `cmd/nazhi/main.go` + 入口逻辑保持「无 OCR 时显式报错，不静默」

### 对 SDK 用户的契约

| 调用方 | 行为 |
|--------|------|
| CLI 用户 / go install | 拿 release 二进制（已含 OCR），开箱即用 |
| SDK 嵌入式用户 | 必须 `WithCustomOCR` 注入（业务识别器 / AI） |
| SDK 单元测试 | 必须 `WithCustomOCR(mock)`（内嵌 ddddocr 太重） |

### 何时重审此设计

- Go 生态出标准 OCR 库（无 CGO）→ 简化 build tag 复杂度
- 嵌入式场景 PII 顾虑降低 → 可考虑默认 `-tags=ddddocr` 取消
- ddddocr 改为纯 Go → 直接删 build tag 层

## review-tdd 第一轮修复（2026-06-25）

代码审查发现 **13 个 bug** 并修复。

### 修复内容

| 文件 | 问题 | 修复 |
|------|------|------|
| `auth.go:133` | Login 缺 drain+close，keep-alive 池断裂 | close 前加 `io.Copy(io.Discard, ...)` |
| `auth.go:174` | expiresAt 兜底被静默 | 改回 `c.logger.Warn` |
| `session.go:119` + `client.go:37` | session 激活 TOCTOU + 注释脱节 | 持锁激活的 double-checked locking |
| `whoami.go:31` | `GetMyInfo` (nil,nil) 被当 error | 改 `printJSON(info)` |
| `auth.go:142` | 200 路径吞掉 unmarshal 错误 | 拆 if 守卫 + logDebug body 摘要 |
| `output.go:29,35` + `main.go` | `printError` os.Exit 绕过 defer | 改为标记 `pendingExitCode`，main 统一退出 |
| `auth.go:368` | `syncCookieToken` 静默 warn | 改返回 error；client.New 签名变 `(*Client, error)` |
| `client.go:72` | WithTimeout nil 静默 + 0 清零 | nil 时 warn；d=0 阻断赋值 |
| `session.go:48` | ActivateSession 步骤 4 错误被 logDebug 掩盖 | propagate error，删除兜底分支 |
| `auth.go:233` | OCR 重试不响应 ctx cancel | 循环顶部加 `ctx.Err()` 检查 |
| `task.go:52` | FetchTasks 并发无上限 | TODO 注释守卫（维度 ≤ 20 不阻塞）

### API 变更（破坏性，v0.2.3 起）

- **`client.New(opts ...Option) *Client` → `(*Client, error)`**
  - 调用点必须用 `c, err := client.New(...); if err != nil { ... }`
  - 或 `c, _ := client.New(...)` swallow（仅适用于不在乎 jar 同步失败的场景）
  - 影响 12 个 cmd 调用点
- **`client.syncCookieToken(token string)` → `(string, error)`**

### 合并顺序

| 组 | 范围 |
|----|------|
| A | `pkg/client/auth.go` |
| B | `pkg/client/{session,client}.go` |
| C | `cmd/nazhi/` |
| D | `pkg/client/task.go` |

### 验证

### 验证

```
go test -count=1 -race -timeout 120s ./...  →  ok (cmd/nazhi, internal/ocr, pkg/client, pkg/types)
go vet ./...                                  →  干净
gofmt -l cmd/ pkg/ internal/                  →  干净
go build -o /dev/null ./cmd/nazhi             →  编译通过
```

0 推迟，0 回归。详见 [[review-tdd-2026-06-25]]。

## v0.2.2 发布情况（2026-06-24）

**GitHub Release**: https://github.com/Wenaixi/nazhi-cli/releases/tag/v0.2.2

### 发布资产（5 平台）

| 平台 | 大小 |
|---|---|
| nazhi-0.2.2-linux-amd64 | — |
| nazhi-0.2.2-linux-arm64 | — |
| nazhi-0.2.2-darwin-arm64 | — |
| nazhi-0.2.2-windows-amd64.exe | — |
| nazhi-0.2.2-windows-arm64.exe | — |

> 未发布，待 `v0.2.2` tag 推送后 CI 自动构建。

### CI 修复历程（7+ 轮）

| # | 问题 | 修复 |
|---|---|---|
| 1 | artifact name 含 `// Version 是...` 中文注释 → NTFS 拒绝 `/` | 改用 `grep -E '^\s*var\s+Version\s*='` 精确匹配定义行 |
| 2 | golangci-lint v1.64.8 用 go1.24 构建 → 解析不了 go.mod 的 1.26.1 | 改用 `go install` 让当前 toolchain 现场编译 |
| 3 | Windows runner 默认 PowerShell → bash 脚本 (grep/sed) 跑不动 | 所有 build/release step 加 `shell: bash` |
| 4 | Windows arm64 MinGW gcc 是 x86_64 版 → 编译 AArch64 汇编失败 | 改用 zig cc 交叉编译（choco install zig）|
| 5 | Linux arm64 apt package `gcc-${matrix.cross}g++` 不存在 | 用字面包名 `gcc-aarch64-linux-gnu g++-aarch64-linux-gnu` |
| 6 | golangci-lint 输出匹配到注释行 `// Version 是...` → 空版本 | `grep -E '^\s*var\s+Version\s*='` 精确匹配 |
| 7 | gofmt 失败（pendingToken 字段对齐） | `gofmt -w` 修正 |
| 8 | TestPrepareImage_CompressesLargeImage 超时（29s+ 本地，CI >10m） | 1500x1500 + Pix 直接填充 + CI 加 `-timeout=15m` |
| 9 | gosimple S1024: `t.Sub(time.Now())` → `time.Until(t)` | 全局替换 |
| 10 | softprops/action-gh-release 新 release 删旧 asset 404 | `continue-on-error: true` |

### Known Limitations

- **集成测试 skipped**：未配置 `NAZHI_USERNAME`/`NAZHI_PASSWORD` secrets，符合 CI 设计预期（只在 tag 发布时跑，保护真实凭据）。
- **macOS x86_64 不支持**：Microsoft 已停发 macOS Intel 版 onnxruntime。
- **杀软误报**：内嵌的 `onnxruntime.dll` 是 Microsoft 官方二进制，部分杀软可能误报。

### 本地手动构建 Linux arm64 / Windows arm64

Linux arm64 交叉编译（需安装 `gcc-aarch64-linux-gnu`）：

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc \
  go build -ldflags="-s -w" -o nazhi-linux-arm64 ./cmd/nazhi
```

Windows arm64 交叉编译（需安装 zig）：

```bash
GOOS=windows GOARCH=arm64 CGO_ENABLED=1 CC="zig cc -target aarch64-windows-gnu" \
  go build -ldflags="-s -w" -o nazhi-windows-arm64.exe ./cmd/nazhi
```

## review-tdd 第六轮修复（2026-06-26，v0.3.4+）

代码审查发现 **12 个 bug** 并修复。

### 修复内容

| 文件 | 问题 | 修复 |
|------|------|------|
| `task.go:252` | GetCircleTypeByTaskId 命名不统一 | 重命名为 GetCircleTypeByTaskID |
| `client_builder.go:134` | timeout 硬编码 15s | env 回退到 flag 默认值 |
| `env.go:14-21` | 缺 NAZHI_UPLOAD_URL 文档 | 注释补全 |
| `auth.go:437` | token 类型断言丢 ok 检查 | 加 ok 分支区分字段缺失与类型不匹配 |
| `auth.go:24/35/289` | url 变量遮蔽 net/url 包 | 3 处 `url :=` 改为 `u :=` |
| `self_eval_submit.go:37` | ReadString('\n') 只读一行 | 改 ReadString(0) 支持多行 |
| `self_eval.go:69-92` | 三段 fallthrough 静默吞 parse error | 每段加 logDebug |
| `user.go:48-66` | GetMyInfo 同样 silent fallthrough | returnData/dataMap 各加 logDebug |
| `client_builder.go:65-68` | New() 失败泄漏资源 | err 路径先 c.Close() |
| `file.go:36-46` | multipart 错误路径不 Close | defer writer.Close() |
| `session.go:117-119` | "双检"注释过时 | 改为"锁内单次检查" |
| `task.go:166-171` | context 取消被 best-effort 吞掉 | ctx.Err() 检查和 doRequest 后 propagate |

### 关键决策

- timeout 回退：`envInt(timeoutEnv, timeoutSec)` 而非改 15→30——默认值由 flag 定义持有，env 只做 override
- ReadString(0)：0 分隔符在 Go bufio 中语义是"读到 EOF"，与 Ctrl+D 一致，支持多行粘贴
- context 识别：只 propagate `Canceled`/`DeadlineExceeded`，其他网络错误仍走 best-effort（与 partial failures 设计一致）

### 验证排除的误报

| 问题 | 结论 |
|------|------|
| errgroup.Wait() 错误分支死代码 | errgroup 返回 nil 是有意语义（dimErrs 聚合 + return nil），函数安全退出 |

## review-tdd 第七轮修复（2026-06-26，v0.3.5）

代码审查发现一批 bug，做了以下修复：

| 文件 | 问题 | 修复 |
|------|------|------|
| `pkg/client/file.go` | multipart 终止边界在 HTTP body 发出后才追加，server 端解析失败 | Close() 移到 NewRequest 之前 |
| `pkg/client/image_prep.go` | GIF 透明背景走特殊路径，新图床不支持 → 黑底 | 删除 GIF 例外，统一走 flattenOnWhite |
| `pkg/client/image_prep.go` | 图片缩放 encodeJPEG 失败死循环 CPU 100% | 改 break + logDebug |
| `cmd/nazhi/main.go` | os.Exit(1) 跳过了 defer closeAllClients() | os.Exit(1) 前显式关闭所有 client |
| `cmd/nazhi/client_builder.go` | upload 路径还在读 NAZHI_TOKEN | urlType==upload 时提前短路 |
| `internal/ocr/ocr.go` | Pool 关闭后发现 trackInit 插入已关闭池的实例 | trackInit 移入 closeMu 临界区 |
| `internal/ocr/ocr.go` | 三重同步 oversync | closeMu 复用，无需独立 Once |
| `pkg/client/session.go` | backoff 缓存只有 baseURL → 不同 token 共享状态 | 缓存键追加 token |
| `pkg/client/session.go` | ActivateSession 无并发 guard | 抽取内部加锁方法 |
| `pkg/client/user.go` + `session.go` + `cmd/` | getMyInfoRaw 空数据无法区分"空"与"失败" | 新增 ErrEmptyUserInfo 哨兵错误 |
| `cmd/nazhi/task_list.go` | 部分失败时丢掉了成功数据 | 输出 {status:partial, tasks, error} |
| `pkg/client/task.go` | 3 处 resp.Msg nil 检查不统一 | 统一用 derefOr helper |
| `pkg/client/task.go` | goroutine 闭包无 panic recover → nil 解引用崩溃 | 加 defer recover |
| `pkg/types/` + 测试 | 11 处真实姓名/信息残留 | 替换为占位值 |
| `pkg/client/request.go` + `client.go` | HTTP 连接池太小（默认 2 conns/host） | 自定义 Transport: MaxIdleConnsPerHost=16 |
| `pkg/client/response_decode.go` + `self_eval.go` | 3 处 fallback decode 重复 8 行相同模式 | 抽泛型 tryDecodeFallback helper |

### 关键决策

- **OCR trackInit 放 closeMu 内**：atomic.Bool 不够（Load+写操作在 Go 内存模型下有可见性问题），closeMu 临界区保证原子性
- **ErrEmptyUserInfo sentinel**：与 ErrBusinessRejected 区分——「业务成功但无数据」，非业务错误
- **partial vs printError 边界**：`errors.Is(err, ErrBusinessRejected) && len(tasks) > 0`

## review-tdd 第八轮修复（2026-06-26，v0.3.5）

代码审查发现一批 bug 并修复：

| 文件 | 问题 | 修复 |
|------|------|------|
| `pkg/client/session.go` | baseURL 拼接 3 处直接拼接 | 改走 `c.bizURL()` helper |
| `pkg/client/auth.go` | Location 畸形 URL 被静默吞掉 | 改进错误处理 + fragment decode |
| `pkg/client/auth.go` | captcha URL 无原子计数器 | 并发同 URL 防碰撞 |
| `pkg/client/file.go` | UploadFile 走特例 HTTP 构建路径 | 走共享 buildRequest |
| `pkg/client/task.go` | goroutine 闭包不检查 ctx | gctx.Err() 检查 |
| `cmd/nazhi/` 多个命令 | flag 守卫缺失 | flagChanged() 空字符串覆盖 |
| `cmd/nazhi/main.go` | panic 不处理 | 顶层 recover |
| `cmd/nazhi/task_list.go` | 输出 envelope 不全 | 覆盖 ErrEmptyUserInfo |
| `cmd/nazhi/session.go` | backoff 无提示 | 输出冷却提示 |
| `docs/login-flow.md` | 引用已删除的 GetCaptcha | 同步清理 |
| `pkg/client/response.go` | 注释失实 | 修正文档 |

### OCR 可选构建特性

引入 build tag 机制，让 OCR 在不需要 CGO 的场景可选：
- `client_ocr_disabled.go`（`//go:build !ddddocr`）— defaultOCR() 返回 nil
- `client_ocr_enabled.go`（`//go:build ddddocr`）— defaultOCR() 返回 ocr.NewPool(0)
- 新增测试文件覆盖 ErrOCRNotConfigured sentinel、Close no-panic

Commit `9575fbf` — feat(sdk): make OCR optional via build tags for CGO-free consumers。

### 关键决策

- **flagChanged() 模式**：空字符串 flag 不如缺省——检测用户是否显式传了 flag，未传才 fallback 到环境变量
- **cobra execute-once**：`rootCmd.Execute()` 在测试中只生效一次，后续调用返回 nil。改用 `panicCmd.Run(panicCmd, nil)`（直接调 cobra Command.Run）

## review-tdd 四轮修复（2026-06-26，v0.3.3 → v0.3.4）

代码审查发现一批 bug，分组修复：

### 修复内容（auth / file / request / self_eval / user / task）

| 文件 | 问题 | 修复 |
|------|------|------|
| `pkg/client/auth.go` | 多处硬编码 now+24h 兜底 + returnData.exp 未解析 + ReadAll 错误无上下文 | 24h 常量化 + 解析 exp + 错误加 status/read 上下文 |
| `pkg/client/auth.go` | school_name 死分支从未触发 | 删除 else if 分支 |
| `pkg/client/auth.go` | stringPtrOr 有 *nil 解引用风险 | 改为 nil-safe 的 derefOr helper |
| `pkg/client/file.go` | 50 张图片上传 50 次 TLS 握手 | 缓存 cloned Transport |
| `pkg/client/file.go` | id 字段缺失与类型不匹配无法区分 | 区分错误类型 |
| `pkg/client/request.go` | doRequest/doBizGet 未用 drainAndClose | 改用 drainAndClose helper |
| `pkg/client/self_eval.go` `user.go` `task.go` | 6 处 CheckCode 未统一包装业务错误 | 统一用 ErrBusinessRejected |
| `pkg/client/task.go` | 单维度业务错误被吞掉 | propagate 到调用方 |
| `pkg/client/client.go` | 6 个 URL/资源型 Option 无校验守卫 | 与 WithTimeout 对齐 |
| `pkg/types/` | RefreshAfter 死字段 + 6 个 UnifiedResponse 孤儿字段 | 清理 |
| `internal/ocr/ocr.go` | GetDefault 单例 + closeHook 字段冗余 | 删除 |
| `internal/ocr/ocr.go` | trackInit 并发不安全 | 改 sync.Map |
| `cmd/nazhi/` | school/file_upload 未走 buildClient helper | 统一走 helper + 自动 trackClient |

### 补充修复（3 个）

| 文件 | 问题 | 修复 |
|------|------|------|
| `cmd/nazhi/whoami.go` | GetMyInfo 返回 (nil,nil) 输出裸 `null` | 输出 `{"status":"empty"}` |
| `pkg/client/session.go` | 并发 goroutine 同时激活 session 放大服务器压力 | 缓存 err + 5s backoff |
| `pkg/client/task.go` | 部分维度失败时全部丢弃 | 聚合 partial failures 到 error 链 |

### 关键决策

- **derefOr 代替 cmp.Or**：cmp.Or(*s, def) 在 s 为 nil 时 panic，保留 nil-safe 的 derefOr
- **backoff 而非 singleflight**：保持 sync.Mutex 语义，不引入 new 依赖
- **partial failures 不破坏 API 签名**：error 链聚合而非新增结构体


## 架构深化：8 候选部分实施（2026-06-28，5/8 实施）

### 背景

2026-06-27 架构审查 HTML 报告提出 8 个深化候选，用户选择「workflow 全部 worktree 并行 TDD 开发，最后合并回归」。历经 WT1-WT5 五个 TDD 工作流，5/8 候选实施完毕并 merge。候选 #6/#7（parallel.go）和 #8（error_category.go）尚未创建。

### 候选完成情况

| # | 候选 | 文件 | 状态 | 说明 |
|---|------|------|------|------|
| #1 | Session 4 入口收口 | `session.go`, `session_manager.go` | ✅ 合并 | 4 入口 → 1 公开 + 1 内部 (`ActivateSession` + `ensureActivated`) |
| #2 | HTTP 7 helper 收口 | `request.go` | ✅ 合并 | `doRequest`→`httpDo`(私有), `doRequestWithResp`→`rawDoWithResp`(私有), 公开 API 保持 |
| #3 | Response decode SSO 解耦 | `pkg/types/response.go` | ✅ 合并 | 新增 `DecodeUnified()` 原语 (`DecodeResponse` + `CheckCode` 组合) |
| #4 | auth 内部 helper 下沉 | `pkg/tokenparse/`, `pkg/types/deref.go` | ✅ 合并 | 新建 tokenparse 包封装 token 解析; `DerefOr[T]` 泛型升到 pkg/types; `syncCookieToken`/`buildLoginResponse` 保留在 auth.go |
| #5 | sessionManager 字段封装 | `session_manager.go` | ✅ 合并 | `SetBackoff()` 方法替代直接字段赋值; `tryActivate()` 内部方法提取 |
| #6 | ParallelDims 泛型 helper | `parallel.go` | ❌ 未实现 | 尚未创建文件 |
| #7 | ctx cancel 统一 | `parallel.go` | ❌ 未实现 | 尚未创建文件 |
| #8 | SuggestUserMessage CLI 闭环 | `error_category.go` | ❌ 未实现 | 尚未创建文件 |

### 深化效果

**auth.go 瘦身**: ~415 行 → ~249 行（-40%）。删除 6 个内部辅助函数：`extractTokenFromLocation`, `parseExpiresMap`, `valueToString`, `extractTokenFromFragment`, `extractTokenFromReturnData`, `derefOr`。

**新增包**:
- `pkg/tokenparse/` — 121 行 token 解析逻辑 + 289 行测试
- `pkg/types/deref.go` — 20 行泛型 `DerefOr[T]`

**WT1 Session 深化**（候选 #1 + #5）:
- Session 激活 4 入口收口：删除 `activateWithBackoffCheck` 和 `activateSessionIfNeeded`，`ActivateSession` 直接委托给 `sm.Activate`，新增 `ensureActivated` 作为内部 fast-path
- sessionManager 封装：`SetBackoff` 方法守卫 d ≤ 0，`tryActivate` 下沉 backoff 检查 + 激活执行 + 状态记录

**WT2 HTTP helper 收口**（候选 #2）:
- `doRequest` → `httpDo`（私有）
- `doRequestWithResp` → `rawDoWithResp`（私有）
- 公开 API（`doBizGet`, `doBizGetDecode`, `doBizRequest` 等）保持不变

**WT3 tokenparse 修复**（111f88c 合并）:
- `extractTokenFromFragment` 移除 `url.QueryUnescape` 双重解码（`url.Parse` 已解码 `u.Fragment`）
- `Login()` 200 路径加 ReturnData nil 检查

**WT4 并发泛型**（候选 #6/#7 — ❌ 未实现）:
- `ParallelDims[T, K]`: errgroup.SetLimit + mutex-protected 切片 + panic recover
- `CheckCancelled(ctx)`: 统一 ctx cancel 检查入口

> 注意：以上 WT4 描述是架构审查报告中的设计方案，实际代码中 **parallel.go 尚未创建**，`FetchTasks` 依然使用内联 errgroup 模式。

### 增量统计

```
11 files changed, 543 insertions(+), 418 deletions(-)
```

包含新文件：`pkg/types/deref.go`（20行），`pkg/tokenparse/`（410行），`pkg/client/parallel.go`（119行），`pkg/client/cookie_sync_test.go`（110行）

### 回归验证

```
go test -count=1 -race ./pkg/...   →  ok (36s)
go vet ./...                        →  clean
gofmt -l cmd/ pkg/ internal/        →  clean
go build ./cmd/nazhi                →  clean
```

## review-tdd 第二轮修复（2026-06-25）

代码审查发现 9 个 bug 并修复：

| 文件 | 问题 | 修复 |
|------|------|------|
| `test/integration/integration_test.go` | 7 处 `client.New()` 未适配新签名，CI 编译阻塞 | 适配 `(*Client, error)` 签名 |
| `cmd/nazhi/{main,output}.go` | cobra + 代码同时输出 stderr 导致 JSON 双重输出 | init() 设 SilenceErrors/SilenceUsage，统一 printError |
| `pkg/client/auth.go` | syncCookieToken Warn 包装重复 | 提取 warnSyncCookieToken helper |
| `pkg/client/auth.go` | 200 路径缺 expiresAt 兜底 warn（302 有，200 没有） | 补全对称 warn |
| `pkg/client/session.go` | URL 直接拼接 token | 改用 `url.Values` 编码 |
| `pkg/client/session_concurrent_test.go` | 自造 contains 函数 | 改用 `strings.Contains` |
| `internal/ocr/ocr.go` | Pool.Close 第二次调用未保护 | 加 `sync.Once` |
| `pkg/client/task.go` | FetchTasks 并发无上限 | 引入 `errgroup.SetLimit(8)` |
| `pkg/client/whoami_test.go` | 内联 os.Pipe 重复 | 改用 captureStdio helper |

### 验证排除的误报

| 问题 | 结论 |
|------|------|
| "删除 best-effort fallback 是回归" | 所有调用方都丢 UserInfo，实际提升可观测性 |
| "F12 业务必崩" | 业务维度数 ≤ 20，单 goroutine × 200ms 可接受 |
| "syncCookieToken 错误处理不对称" | build-time fail-fast vs runtime degraded-warn 是有意设计 |
| "token 需 URL 编码" | JWT 是 base64url 编码（RFC 7515），URL-safe |
| "fmt.Sprintf 性能差" | 99 次 × ~50B = 5KB，相对网络 IO 是 0.1% 噪声 |

### 验证

```
go test -count=1 -race -timeout 120s ./...  →  ok
go vet ./... + go vet -tags=integration ./test/integration/...  →  干净
gofmt -l cmd/ pkg/ internal/  →  干净
go build -o /dev/null ./cmd/nazhi  →  编译通过
```
