# 架构总览

## 双层架构

```
┌─────────────────────────────────────┐
│  cmd/nazhi/  (CLI 层 - cobra 命令)   │
│  - 薄壳层：参数解析 + JSON 输出      │
│  - 18 个源文件 (含 client_builder /  │
│    opt_builder / lifecycle / output)│
└────────────┬────────────────────────┘
             │ 调用
             ↓
┌─────────────────────────────────────┐
│  pkg/client/  (SDK 层 - 核心业务)    │
│  - Option 模式构造 Client            │
│  - 13 个公开方法 (含 Close)          │
│  - HAR 对齐的 Session 激活           │
│  - Pool 多实例 OCR 引擎              │
│  - bizURL helper (避免裸 baseURL 拼) │
└────────────┬────────────────────────┘
             │ 使用
             ↓
┌─────────────────────────────────────┐
│  pkg/tokenparse/  (SSO token 解析)    │
│  - ExtractFromLocation / ReturnData  │
│  - token 过期时间兜底 24h             │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  pkg/types/  (共享类型)              │
│  - LoginRequest/Response, Task, ...  │
│  - UnifiedResponse 泛型辅助          │
│  - DerefOr[T] 安全解引用             │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  internal/  (内部包)                 │
│  - internal/ocr/  跨平台 ddddocr     │
│  - internal/version/  版本号        │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│  test/integration/  (真实环境 + HAR)  │
│  - TestReal_FullChain  端到端        │
│  - HAR 驱动测试 + 回归测试           │
└─────────────────────────────────────┘
```

## 架构深化候选（已实施 #1~#5）

`arch-tdd` 流程识别出 8 个架构深化候选，前 5 项已落地：

| # | 候选 | 落地位置 | 解决的问题 |
|---|------|----------|------------|
| #1 | **Session 收口** | `pkg/client/session.go` 新增 `sessionManager` 状态机 | `ActivateSession` 原本散落 4 处状态字段，被 6 个方法同时读写；收口后 `tryActivate` / `RecordFailure` / `RecordSuccess` 单一入口 |
| #2 | **HTTP helper 私有化** | `request.go` 的 `httpDo` / `rawDoWithResp` 提取共享 `do()` 核心 | SDK 业务方法此前混用 3 套 HTTP 调用路径，统一走 `do()` |
| #3 | **`DecodeUnified` 原语化** | `pkg/types/response.go` `DecodeUnified()` (原语) + `DecodeReturnData[T]` / `DecodeDataList[T]` / `DecodeDataMap[T]` | SDK 侧 fallback 解码统一走 `tryDecodeFallback` helper |
| #4 | **tokenparse 包** | 新包 `pkg/tokenparse/tokenparse.go` | `auth.go` 原内嵌的 `extractTokenFromLocation` / `extractTokenFromReturnData` 抽到独立包，附带 `DefaultTokenTTL=24h` 兜底、移除 `ErrLocationParseFailed` 死 sentinel |
| #5 | **`DerefOr[T]` 泛型** | `pkg/types/deref.go` 升到 types 包供全包复用 | 各处重复的 `if ptr != nil { *ptr } else { def }` 三元模式 |

**未实施的 #6/#7/#8**（仍在候选池）：
- `#6 ParallelDims` 泛型 helper（`FetchTasks` 仍用内联 `errgroup`）
- `#7 error_category.go` 错误分类
- `#8 pkg/client/error.go` 错误码集中定义

## 目录结构

```
nazhi-cli/
├── cmd/
│   └── nazhi/                # CLI 入口
│       ├── main.go           # cobra root
│       ├── login.go          # SSO 登录
│       ├── school.go         # 查询学校 ID
│       ├── session.go        # 业务 Session
│       ├── whoami.go         # 用户信息
│       ├── task_list.go      # 任务列表
│       ├── task_submit.go    # 任务提交
│       ├── self_eval_*.go    # 自我评价
│       ├── file_upload.go    # 文件上传
│       ├── version.go        # 版本信息
│       ├── completion.go     # shell 自动补全
│       ├── env.go            # 环境变量加载
│       └── output.go         # JSON 输出
├── pkg/                      # 公开 SDK
│   ├── client/
│   │   ├── auth.go              # Login / GetSchoolID / InitSession
│   │   ├── session.go           # ActivateSession + sessionManager
│   │   ├── task.go              # FetchTasks / SubmitTask / GetDimensions
│   │   ├── self_eval.go         # SubmitSelfEvaluation / Query...
│   │   ├── user.go              # GetMyInfo
│   │   ├── file.go              # UploadFile
│   │   ├── image_prep.go        # 上传前图片压缩
│   │   ├── request.go           # HTTP 封装 (do/httpDo/rawDoWithResp)
│   │   ├── client.go            # Client struct + Option
│   │   ├── client_ocr_enabled.go   # //go:build ddddocr
│   │   ├── client_ocr_disabled.go  # //go:build !ddddocr
│   │   ├── cookie_sync.go       # syncCookieToken / buildLoginResponse
│   │   ├── errors.go            # 哨兵错误 (10 个)
│   │   └── *_test.go            # 单元测试 (~40 文件)
│   ├── tokenparse/              # SSO token 解析
│   │   ├── tokenparse.go        # ExtractFromLocation / ReturnData
│   │   └── tokenparse_test.go
│   └── types/
│       ├── types.go             # 领域类型
│       ├── response.go          # UnifiedResponse + 泛型辅助
│       └── deref.go             # DerefOr[T] 安全解引用
├── internal/                  # 内部包
│   ├── ocr/
│   │   ├── ocr.go              # OCR/Pool + Windows DLL 降级 + 临时目录清扫
│   │   ├── onnx_win_amd64.go   # //go:build windows && amd64
│   │   ├── onnx_win_arm64.go   # //go:build windows && arm64
│   │   ├── onnx_lin_amd64.go   # //go:build linux && amd64
│   │   ├── onnx_lin_arm64.go   # //go:build linux && arm64
│   │   ├── onnx_mac_arm64.go   # //go:build darwin && arm64
│   │   └── models/             # 嵌入模型 + 5 平台原生库
│   └── version/
│       └── version.go          # 版本号
├── test/
│   └── integration/           # 集成测试 (build tag=integration)
├── docs/                      # 文档中心
│   ├── architecture.md         # 本文件
│   ├── cross-platform-ocr.md
│   ├── login-flow.md
│   ├── env-vars.md
│   ├── har-testing.md
│   ├── cli/
│   └── sdk/
├── .github/workflows/ci.yml   # CI
├── CLAUDE.md                  # 项目记忆（git 忽略）
├── README.md
├── CONTRIBUTING.md
├── CHANGELOG.md
├── LICENSE
├── Makefile
├── go.mod
└── go.sum
```

## 关键架构决策

### 1. Option 模式构造 Client

每个 Client 拥有独立的 HTTP cookie jar，天然并发安全。`client.New(opts...)` **返回 `(*Client, error)`**（v0.2.3 起破坏性变更，因 cookie jar 同步可能失败）。

### 2. 跨平台 OCR

`internal/ocr/` 5 个 build-tag 隔离的 `onnx_*.go` 文件嵌入对应平台的 onnxruntime：
- windows/amd64
- windows/arm64
- linux/amd64
- linux/arm64
- darwin/arm64

> Windows DLL 占用降级 + 启动清扫历史 temp 目录 — 详见 `docs/cross-platform-ocr.md`。

### 3. OCR 引擎共享（Pool 实例）+ 可选构建

`client.New` 默认行为取决于构建标签：
- `-tags ddddocr`：构造 `ocr.NewPool(0)`（懒加载单实例，开箱即用）
- 无 `-tags ddddocr`：`c.ocr = nil`，Login() 返回 `ErrOCRNotConfigured`，需用 `WithCustomOCR` 注入

Pool 实例共享 ONNX session，模型只解压一次。ONNX session 非线程安全，
`NewPool(n)` 预热 n 个独立 session 支持 n 路真并发（每实例约 50MB）。

> 历史说明：早期版本曾提供 `ocr.GetDefault()` 进程级单例，但生产代码无
> 调用方，已在 v0.3.4 删除。

### 4. HAR 验证的 Session 激活

必须按顺序执行 4 步（`/` + `getMenu` + `getMenu` + `getMyInfo`），否则业务接口返回空。

**Session 收口**：原散落的 4 处状态字段被收口到 `pkg/client/session.go` 的 `sessionManager` 状态机
（架构深化候选 #1）。`tryActivate` / `RecordFailure` / `RecordSuccess` 是单一入口；
backoff 缓存键含 token（不同 token 不共享冷却状态）；`SetBackoff` 加 `sm.mu.Lock` 消除 data race。

### 5. 双重 Token 注入

`WithToken()` 同时写 Header 和 Cookie，匹配服务器双重认证要求。
`syncCookieToken` 把 `baseURL` 在 `New()` 阶段预解析到 `c.baseURLParsed`，
避免每次调用 `url.Parse`（性能优化 F6 + cleanup-url-helper 删 3 个 1-line 同构 helper）。

### 6. 上传安全隔离

`UploadFile` 使用独立 `http.Client`（无 cookie jar + 禁用重定向），防止 SSO Cookie 泄露到文件服务器。

### 7. 任务 Payload 透传

`TaskSubmitPayload` 29 字段不裁剪不处理，调用方传什么发什么。

### 8. 统一响应体解析

`UnifiedResponse` + 泛型辅助 `DecodeUnified()`（原语）/ `DecodeReturnData[T]` /
`DecodeDataList[T]` / `DecodeDataMap[T]` / `CheckCode()`（code==1 为成功）。
SDK 侧 fallback 解码统一走 `request.go` 的 `tryDecodeFallback` helper（架构深化 #2/#3）。

### 8.5. token 解析下沉到 pkg/tokenparse

`auth.go` 原内嵌的 `extractTokenFromLocation` / `extractTokenFromReturnData` 抽到
新包 `pkg/tokenparse/tokenparse.go`（架构深化候选 #4）：
- `ExtractFromLocation(location string)` — 从 302 Location 头解析 token
- `ExtractFromReturnData(raw json.RawMessage)` — 从 ReturnData JSON 字节解析 token
- 两者均返回 `(token string, expiresAt time.Time, err error)` 三元组
- 过期时间兜底 `DefaultTokenTTL = 24 * time.Hour`（server 不带 expires_in/exp 时）
- 同步删除 `ErrLocationParseFailed` 死 sentinel（曾因未用 `%w` 链入导致 `errors.Is` 永不命中）

`auth.Login` 两路径（200 JSON / 302 Location）共用此包，保证 token + expiresAt 提取一致。

### 9. HAR 驱动测试

把真实抓包作为 fixture 喂给 mock server，无需期末数据就能测任务流。

### 10. 输出通道例外（避免误判 bug）

CLI 严格遵循「stdout = JSON 输出 / stderr = JSON 错误 + verbose 日志」双通道契约，
但以下路径直写 stderr 是**有意的设计**，不是绕过：

| 路径 | 位置 | 意图 |
|------|------|------|
| `printPrompt` | `cmd/nazhi/output.go` | stdin 交互提示（如 self-eval submit 的「请输入评价: 」），**不**受 verbose 守卫，但受 `isTerminalStdin()` + `quiet` 守卫 |
| `c.logger.Warn` 资源警告 | `pkg/client/*.go`（如 `pool.Close` 失败、`http.CloseIdleConnections` 异常、`Login` expiresAt 兜底告警） | 直接走用户注入的 slog handler，不走 printError（保持 SDK 纯净，不引入 cmd 依赖） |
| backoff 冷却提示 | `cmd/nazhi/session.go` | 捕获 `ErrSessionBackoff` 时输出 `{"status":"cooldown","message":"..."}` 让脚本感知等待 |

**为什么需要文档化**：维护者看到直写 stderr 第一反应是「绕过 printError，应重构」。
但 `printPrompt` 走 printError 会污染 stderr 错误流（混进 JSON envelope），
走 printVerbose 用户没加 `-v` 就看不到提示——只有直写 stderr 才能同时满足
「用户可见 + 不污染错误流」两个约束。

### 依赖关系

```
internal/ocr (dddocr + onnxruntime)  ← 跨平台二进制 (5 build tag + Windows 降级 + 启动清扫)
        ↓
pkg/client  ← Option 模式 + 13 个公开方法 + sessionManager
        ↓
pkg/tokenparse  ← SSO token 解析
        ↓
pkg/types  ← DerefOr[T] + UnifiedResponse 泛型辅助
        ↓
cmd/nazhi  ← cobra 命令
        ↓
test/integration  ← 真实环境 + HAR fixtures
```

## 数据流

### 登录流（并发优化）

```
用户 → nazhi login
     → Login(ctx, LoginRequest)
     → InitSession (GET /uiStudentLogin/login)  ← 串行前置，必须最先建 JSESSIONID
     → errgroup.WithContext:                     ← 无数据依赖，并发
         ├─ GetSchoolID (POST getSchoolIdByStudentNumber)
         └─ ocrRecognizeWithRetry (GET kaptcha.jpg + 1×99)
     → validateCaptcha (POST /validateCaptcha)  ← 依赖 OCR 结果，串行
     → Login POST (validate)                     ← 200 JSON 优先 / 302 fallback
     → tokenparse.ExtractFrom* → (token, expiresAt)
     → buildLoginResponse → syncCookieToken (X-Auth-Token)
     → 返回 LoginResponse { Token, ExpiresAt, RawData }
```

### Session 激活流（4 步 HAR 对齐）

```
1. GET /                      (初始化后端 Session)
2. GET /api/studentInfo/getMenu  (Referer: /homepage?token=xxx)
3. GET /api/studentInfo/getMenu  (Referer: /home)
4. GET /api/studentInfo/getMyInfo (返回完整 UserInfo)
```

### 任务拉取流

```
FetchTasks(token)
  → ActivateSession (4 步)
  → getDimensions (返回 7 个维度)
  → 对每个非 0 维度:
      getCircleStatistics?dimensionId=X
  → 聚合所有任务，注入 dimensionName
```

### 任务提交流

```
SubmitTask(token, payload)
  → POST /api/studentCircleNew/addCircle
  → 29 字段 payload 透传
  → 返回 { code: 1, insertID: 12345 }
```
