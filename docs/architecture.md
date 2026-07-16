# 架构总览

## 双层架构

```
┌──────────────────────────────────────────────────────────┐
│  cmd/nazhi/  CLI 层：cobra 命令薄壳                       │
│  - 参数解析 + env fallback + JSON 输出                    │
│  - 顶层 panic recover（debug.Stack）+  LIFO 资源清理       │
│  - main/client_builder/opt_builder/                         │
│    lifecycle/output/env/parents + 各子命令                  │
└────────────────────┬─────────────────────────────────────┘
                     │ 调用
                     ↓
┌──────────────────────────────────────────────────────────┐
│  pkg/client/  SDK 层：核心业务                            │
│  - Option 模式构造                                         │
│  - 公开方法：Login / ActivateSession / FetchTasks /        │
│    SubmitTask / SubmitSelfEvaluation / GetHonorList /      │
│    UploadFile / DownloadFile / ...                         │
│  - HAR 对齐 Session 激活 + sessionManager 状态机          │
│  - Pool 多实例 OCR 引擎 + ddddocr/!ddddocr build tag 分发  │
│  - 哨兵错误（errors.Is 精确分支）                          │
└─────────┬─────────────────────────┬──────────────────────┘
          │ 使用                    │ 使用
          ↓                         ↓
┌─────────────────────┐    ┌──────────────────────────────┐
│  pkg/tokenparse/    │    │  pkg/types/                  │
│  SSO token 解析     │    │  领域类型 + 泛型解码         │
│  Location /         │    │  UnifiedResponse +           │
│  ReturnData →       │    │  DecodeReturnData[T] etc.   │
│  (token, exp)       │    │  DerefOr[T] 安全解引用        │
└─────────────────────┘    └──────────────────────────────┘

┌─────────────────────┐    ┌──────────────────────────────┐
│  internal/ocr/      │    │  test/integration/            │
│  跨平台 ddddocr     │    │  真实环境 + HAR 驱动         │
│  5 平台 onnxruntime │    │  + PII SHA-256 守卫          │
└─────────────────────┘    └──────────────────────────────┘
```

设计取舍：三个公开包各自只做一件事，不互嵌业务逻辑。
- `pkg/client` 偶发性依赖 `pkg/types` 与 `pkg/tokenparse`
- `pkg/types` 与 `pkg/tokenparse` 互不依赖（第三方可单独 import 不拖整个 SDK）

> `pkg/client/parallel.go` 与 `pkg/client/error_category.go` **已实施**。
> `FetchTasks` 仍用内联 `errgroup`（业务复杂性待后续迁移），但 `ParallelDims[T]` 泛型 helper 与 `ClassifyError` 分类枚举已可复用。

## 关键架构决策

### 1. Option 模式构造 Client

```go
func New(opts ...Option) (*Client, error)
```

每个 `*Client` 实例独立的 cookie jar，**天然并发安全**。构造函数返回 `(*Client, error)`——
`error` 在 `syncCookieToken` 失败时返回（典型场景：自定义 `*http.Client` 的 `Jar` 不是 `*cookiejar.Jar`）。

**公开 Option**：

| Option | 类型 | 默认 | 拒绝无效值 |
|---|---|---|---|
| `WithSSOBase` / `WithBaseURL` / `WithUploadURL` | string | SDK 默认 URL | `""` 拒绝并 warn |
| `WithTimeout` | time.Duration | `15s` | `<=0` / `c.http==nil` 拒绝 |
| `WithHTTPClient` | `*http.Client` | 默认带 cookie jar | `nil` 拒绝 |
| `WithLogger` | `*slog.Logger` | stderr WARN | `nil` 拒绝 |
| `WithToken` | string | 无 | 空 / 纯空白拒绝 |
| `WithCustomOCR` | `CaptchaRecognizer` | `ocr.NewPool(0)`（含 OCR 构建）/ nil | `nil` 拒绝 |
| `WithOCRConcurrency` | int | `min(4, NumCPU)`（含 OCR） | `<=0` 拒绝 |
| `WithSessionBackoff` | time.Duration | `5s` | `<=0` 拒绝 |
| `WithSubmittedPageSize` | int | `100` | `<=0` 拒绝；调整 GetSubmittedCircles 分页大小 |

`withDurationGuard` 是 Option 构造工厂（拒绝 `<0` / `=0` 后调 setter），消除 WithTimeout / WithSessionBackoff 中重复的守卫逻辑。

### 2. 自定义 Redirect Handler

```go
var noRedirect = func(_ *http.Request, _ []*http.Request) error {
    return http.ErrUseLastResponse
}
```

`http.Client.CheckRedirect = noRedirect` **不自动跟随 302**，因为 SSO 登录成功的 JWT token 在 302 的 `Location` 头里。`noRedirect` 是包级共享变量。

### 3. 双重 Token 注入

业务服务器要求 `X-Auth-Token` 同时存在于 Header 和 Cookie。

`WithToken` 同步写：

- Header `X-Auth-Token`（`bizHeaders` / `ssoHeaders` 构造时设置）
- Cookie `X-Auth-Token` 由 `syncCookieToken` 调 `jar.SetCookies` 写入

`baseURL` 在 `New()` 末尾预解析到 `c.baseURLParsed`（`atomic.Pointer[url.URL]`，F3 修复），热路径无锁。
直接构造 `Client{}` 绕过 `New()` 时懒解析一次并 CAS 写回。

**为什么必须双重**：业务 API 用 cookie 鉴权而非 Authorization 头，仅设 Header 会返回空 dataList。
Cookie 同步失败（如自定义 `Jar` 不是 `*cookiejar.Jar`）会让 `New()` 返回 error，让调用方立即感知。

### 4. HAR 验证的 Session 激活

`ActivateSession` **必须按以下 4 步顺序**（HAR 抓包验证）：

```
1. GET /                                    (建后端 Session)
2. GET /api/studentInfo/getMenu             (Referer: /homepage?token=xxx)
3. GET /api/studentInfo/getMenu             (Referer: /home)
4. GET /api/studentInfo/getMyInfo           (返回完整 UserInfo)
```

跳过任何一步都会让后续接口返回空数据。

**架构深化**：

- **4 入口收口**：删除 `activateWithBackoffCheck` + `activateSessionIfNeeded`，`ActivateSession` 直接委托给 `sm.Activate`
- **`sessionManager` 封装**：`SetBackoff` 加 `sm.mu.Lock` 消除 d≤0 race + 与 WithTimeout 守卫对称
- **`tryActivate` 下沉**：`backoff 检查 + 激活执行 + 状态记录` 单一入口，state mutation 不再分散到 6 处
- **token 隔离的 backoff 缓存**：上次失败的 token 不同不会污染新 token
- **持锁 4 步**：cookie jar 是 Client 级别共享资源，串行持有 `sm.mu` 写入避免竞态

### 5. Login 多图多试 OCR

`InitSession → [GetSchoolID ‖ ocrRecognizeWithRetry] (errgroup) → validate → 200/302 fallback`

```
1. InitSession (串行前置，必须最先建立 JSESSIONID)
2. errgroup.WithContext 并发：
   ├─ GetSchoolID (仅当 req.SchoolID 为空)
   └─ ocrRecognizeWithRetry (GET kaptcha.jpg + 安全调用 Recognize)
3. POST /validate（validateCaptcha 已在 OCR 循环内部完成）
   ├─ 200 路径 → tokenparse.ExtractFromReturnData
   └─ 302 fallback → tokenparse.ExtractFromLocation
5. buildLoginResponse → syncCookieToken (X-Auth-Token)
```

**OCR 重试策略**：单图 OCR 1 次（ddddocr 对同图确定性，重试无意义），失败换新图，最多 9 张图。常量 `maxOCRImagesTotal=9`。

**并发安全**：早期 CallStep 严格排序，之后改用 mutex 保护的状态变量，**支持 Login 并发**（不影响正确性，但 OCR 引擎一次只能识别一张图，所以真并发看 Pool concurrency）。

**token 解析下沉到 pkg/tokenparse**（架构深化 #4）：`ExtractFromLocation` / `ExtractFromReturnData` 两个公开函数，`auth.go` 两条路径（200/302）都走。`DefaultTokenTTL=24h` 兜底 + `ErrLocationParseFailed` 死 sentinel 删除。

### 6. OCR 跨平台 + Pool 多实例（三轮修复）

Build tag 隔离的 `onnx_*.go` 文件嵌入对应平台的 onnxruntime：

| GOOS | GOARCH | 文件 |
|---|---|---|
| windows | amd64 / arm64 | `onnx_win_amd64.go` / `onnx_win_arm64.go` |
| linux | amd64 / arm64 | `onnx_lin_amd64.go` / `onnx_lin_arm64.go` |
| darwin | arm64 | `onnx_mac_arm64.go` |

每文件 `var OnnxRuntimeDLL []byte`，编译时按 `(GOOS, GOARCH)` 只取一份嵌入二进制。
Microsoft onnxruntime v1.25.0 已停发 macOS x86_64，**不支持**。

**三轮 Windows 修复**：

| 轮次 | Commit | 解决的问题 |
|---|---|---|
| 第 1 轮 | `5ff0ea8` | **Windows DLL 占用降级**：`Close` 时删 `onnxruntime.dll` 因 `LoadLibrary` 句柄未释放被拒（`Access is denied`），抽 `cleanupTempDir`，对 Windows 两类 errno（`ERROR_ACCESS_DENIED=5` / `ERROR_SHARING_VIOLATION=32`）降级返 nil |
| 第 2 轮 | `a81c9f3` | **GOOS 守卫**：上一轮注释承诺「非 Windows 永远 false」但代码不保证（Linux errno 5=EIO / 32=EPIPE 也会命中），加 `goosFn` 注入点 + `runtime.GOOS == "windows"` 守卫 |
| 第 3 轮 | `7d5dd65` | **启动时清扫**：每次 `extractModels` 建好本进程目录后，best-effort 扫 `%TEMP%` 下其他 `nazhi-cli-ocr-*` 旧目录——能删的（已退出进程）删掉，删不掉（其他运行实例）跳过 |

每轮都加了单元测试 + 集成测试。详见 [cross-platform-ocr.md](cross-platform-ocr.md)。

### 7. OCR 可选构建（双层 build tag）

`internal/ocr` 依赖 `onnxruntime_go` **强制 CGO**。为兼顾 CLI 开箱即用与 CGO-free 消费者，引入两层 build tag 解耦：

#### 第一层：`ddddocr` — 启用 OCR 引擎
- `client_ocr_enabled.go`（`//go:build ddddocr`）：导入 `internal/ocr`，`defaultOCR()` 返回 `ocr.NewPool(n)`
- `client_ocr_disabled.go`（`//go:build !ddddocr`）：零依赖纯 Go，`defaultOCR()` 返 nil，`Login()` 立即 `ErrOCRNotConfigured`

#### 第二层：`ddddocr_embed` — 嵌入模型文件
- `ocr_embed.go`（`//go:build ddddocr_embed`）：`//go:embed` 嵌入 ONNX 模型 + 字符集，开箱即用
- `ocr_noembed.go`（`//go:build !ddddocr_embed`）：所有嵌入变量为 nil，必须 `WithOCRModelDir` 提供外部模型目录
- 5 个 `onnx_*.go`（`//go:build ... && ddddocr_embed`）：各平台原生库仅在 `ddddocr_embed` 下嵌入

| 构建命令 | `ddddocr` | `ddddocr_embed` | 行为 | 场景 |
|---|---|---|---|---|
| `go build` | ❌ | ❌ | CGO-free，`c.ocr=nil`，需 `WithCustomOCR` | 嵌入式 / 禁 CGO 消费者 |
| `go build -tags=ddddocr` | ✅ | ❌ | OCR 启用，但无内嵌模型，需 `WithOCRModelDir` 外部目录 | 共享模型文件 / 瘦二进制 |
| `go build -tags=ddddocr,ddddocr_embed` | ✅ | ✅ | 完整 OCR，开箱即用 | CLI release / 服务端 |

> **重要** CI 与 Makefile 的 release 构建必须 `-tags=ddddocr,ddddocr_embed`，否则发布的二进制无内嵌模型，
> 用户需额外提供模型目录才能使用。（v0.3.5 同类事故的延续。）

### 8. 统一响应体解析（泛型）

`pkg/types/response.go` — 平台所有 API 返回统一 JSON 结构。泛型解码提供类型安全：

```go
resp, err := types.DecodeResponse(bodyBytes)        // UnifiedResponse
if err := types.CheckCode(resp); err != nil { /* code≠1 → *BusinessError */ }

userInfo, err := types.DecodeReturnData[types.UserInfo](resp)
tasks, err := types.DecodeDataList[types.Task](resp)
selfEval, err := types.DecodeDataMap[types.SelfEvalStatus](resp)
```

- `DecodeReturnData[T]`（single object from `returnData`）
- `DecodeDataList[T]`（array from `dataList`）
- `DecodeDataMap[T]`（single object from `dataMap`）
- `CheckCode` 检查 `code==1`；否则返 `*BusinessError{Code, Msg}` 供 `errors.As` 精细分支

SDK 侧 fallback 解码统一走 `pkg/client/request.go` 的 `doBizGetDecode`——先试 `returnData` 再试 `dataList`，首个成功非 nil 的返回。

### 9. 文件上传 / 下载安全隔离

`UploadFile` 用独立的 `newCleanClient`（**无 cookie jar + 禁用重定向**），杜绝 SSO 鉴权头泄露到文件上传公共服务。`DownloadFile` 同样走无 token 路径，但需要跟随重定向（SSO 域 → FastDFS 存储域），通过白名单限制仅 `.nazhisoft.com` / `nazhisoft.com` 域可跟随。

| 行为 | 原因 |
|---|---|
| **不发任何 Token/Cookie/Authorization** | 文件服务器 `doc.nazhisoft.com` 是独立公共服务，发送业务 token 反而触发风控 |
| **上传独立 `http.Client`** | 即使 `c.http.Jar` 有 Cookie，上传请求也不带 |
| **下载跟随重定向 + 白名单** | `DownloadFile` 跟随 302 到 FastDFS，但仅允许 `.nazhisoft.com` / `nazhisoft.com` 同域；防止 SSRF |
| **共享 Transport 连接池** | 每张图 Clone 一次（O(1) struct copy + 重置 idle pool），N 张图批量上传只需 1 次 DNS+TCP+TLS 握手 |
| **上传前自动预处理** | 任意格式 → JPG + 透明合成 + 缩放/质量级联 → ≤ 5MB |
| **下载 0 字节自愈** | 服务端返回 0 字节时删除半成品文件，不留垃圾 |

**域隔离细节**：`syncCookieToken` 只在 `c.baseURL` 域写 `X-Auth-Token`，而 `UploadFile` 走 `c.uploadURL` 域（独立文件服务器）。即使 `uploadURL` 与 `baseURL` 指向同一主机（自定义部署），同步到 baseURL 的 cookie **不会**被上传请求携带（newCleanClient 无 jar）。调用方仍应注意不要在业务 Client 的 baseURL 域上传敏感文件。
`DownloadFile` 入口在 `ssoBaseURL` 域（`common/attachment/getImg?id=X`），收到 302 后跟着 Location 走到 `doc.nazhisoft.com` 存储域。两个域都不发送任何鉴权头。

### 10. HAR 驱动测试

`test/integration/` 下用真实抓包 fixture 喂 mock server（`httptest.Server`），无需期末数据就能测任务流。

**PII 守卫 SHA-256 哈希方案**：
- 守卫表只存 PII 的 **SHA-256 hex 摘要**（单向不可逆）
- 扫描时算 hash 查表，命中即报错
- 早期守卫曾用 PII 本身自检（`"用 PII 防御 PII"`），结果守卫文件自身成了新的泄露源

详见 [har-testing.md](har-testing.md)。

### 11. CLI 输出通道

```
stdout = JSON 输出（printEnvelope）
stderr = JSON 错误（printError） + verbose 日志（printVerbose）
```

**例外**（直写 stderr，**有意设计**）：

| 路径 | 位置 | 意图 |
|---|---|---|
| `printPrompt` | `cmd/nazhi/output.go` | stdin 交互提示（self-eval submit「请输入评价」），不受 verbose 守卫但受 quiet + `isTerminalStdin()` 守卫 |
| `c.logger.Warn` 资源警告 | `pkg/client/*.go`（如 `Pool.Close` 失败、`Login` expiresAt 兜底） | 走用户注入的 slog handler，**不**走 printError |
| panic stack 输出 | `cmd/nazhi/main.go` | 顶层 panic recover 时 `debug.Stack()` 输出到 stderr 辅助调试 |
| backoff 冷却提示 | `cmd/nazhi/session.go` | 捕获 `ErrSessionBackoff` 时输出 `{"status":"cooldown", "message":"..."}` 让脚本感知 |

**为什么需要文档化**：直写 stderr 易被误判为「绕过 printError 应重构」。
但 `printPrompt` 走 printError 会污染错误流，走 printVerbose 用户没加 `-v` 看不到——只有直写 stderr 同时满足「可见 + 不污染错误流」。

### 12. 顶层 panic recover

```go
// cmd/nazhi/main.go
defer func() {
    if r := recover(); r != nil {
        markError()                    // 设 pendingExitCode=1
        printError(fmt.Errorf("内部错误: %v", r))
        fmt.Fprintln(os.Stderr, "panic stack trace:")
        os.Stderr.Write(debug.Stack())
    }
}()
```

- panic 走与正常 error 相同的 exit code 1（与 Go runtime 默认 2 区分 panic vs user error）
- `debug.Stack()` 写到 stderr 辅助生产定位
- 之后 fall through 到 `pendingExitCode.Load() != 0` 分支，调用 `closeAllClients()` LIFO 释放资源
- `os.Exit(1)` 前**显式**调用 `closeAllClients()`（因为 os.Exit 跳过 deferred functions）

### 13. CLI 资源生命周期

`cmd/nazhi/lifecycle.go` 中：

- `pendingClients` 是 `[]*client.Client`，用 `sync.Mutex` 保护 —— 多个 goroutine 通过 `trackClient` 追加注册
- `closeAllClients()` 是正向遍历顺序（FIFO），与构造顺序一致。Close 本身幂等，无需反向遍历
- `Close` 是幂等的（多次调用安全）

这是为什么 `printError` 不直接 `os.Exit`：必须让 `defer closeAllClients()` 跑完。

## 数据流

### 登录流（并发优化）

```
用户 → nazhi login
     → Login(ctx, LoginRequest)
     → InitSession (GET /uiStudentLogin/login)         ← 串行前置
     → errgroup.WithContext:                           ← 无数据依赖，并发
         ├─ GetSchoolID (POST getSchoolIdByStudentNumber)
         └─ ocrRecognizeWithRetry (GET kaptcha.jpg + 1×9)
     → Login POST (validate)  ← validateCaptcha 已在 OCR 循环内部完成
         ├─ 200 JSON 优先 → tokenparse.ExtractFromReturnData
         └─ 302 Location fallback → tokenparse.ExtractFromLocation
     → buildLoginResponse → syncCookieToken
     → 返回 LoginResponse { Token, ExpiresAt }
```

### Session 激活流（HAR 4 步）

```
c.ActivateSession(ctx, token)
  → sm.Activate
    → sm.mu.Lock
    → DCL fast-path：sm.LoadToken() == token ? → 返 cachedUserInfo
    → tryActivate:
        → ctx.Err() 检查（优先于 backoff，避免掩盖）
        → isBackoffHit(token) ? → 返 ErrSessionBackoff
        → activateFn(ctx, token) (持锁 4 步)
            → GET /
            → GET getMenu (Referer: /homepage?token=xxx)
            → GET getMenu (Referer: /home)
            → GET getMyInfo
        → RecordFailure OR RecordSuccess
    → sm.mu.Unlock
```

### 任务拉取流（并发 + 错误聚合）

```
c.FetchTasks(ctx, token)
  → fetchDimensions  (ActivateSession + getDimensions)
  → for each dim (skip dim.ID == 0 "全部" 维度):
      errgroup.SetLimit(min(len(dim), 8)):
          g.Go(func():
              gctx.Err() 检查
              fetchTasksForDimensionSafe (defer recover panic)
                  → GET getCircleStatistics?dimensionId=X
                  → DecodeResponse + CheckCode
                  → DecodeDataList[types.Task]
                  → 注入 dimensionName
              appendLocked(&mu, &allTasks, tasks...)  // mutex 串行合并
          )
  → g.Wait()
  → 聚合 dimErrs:
      - 全失败 + ctx cancel → 返 ErrRetryable
      - 全失败 + 业务错 → 返 ErrBusinessRejected
      - 部分失败 + 有 partial tasks → 返 (tasks, ErrBusinessRejected 包装含 ErrRetryable)
      - 部分失败 + 无 tasks → 返 (nil, joined errors)
```

### 任务提交流

```
c.SubmitTask(ctx, token, payload)
  → 检查 payload.CircleTaskID && payload.CircleTypeID 必填
  → doBizAndDecode + POST /api/studentCircleNew/addCircle
  → 返回 *TaskResult { Code, Msg }
```

## 测试架构

```
pkg/client/*_test.go            单元测试（httptest.Server mock 服务端，验证 HTTP 方法/路径/头/body/调用顺序/错误路径/并发隔离/cookie jar 独立性）
pkg/tokenparse/*_test.go         tokenparse 单元测试
test/integration/har_*_test.go  HAR 驱动 + PII 守卫（SHA-256 hash）
test/integration/verify_*_test.go    //go:build verify — CLAUDE.md 不能被 git track
test/integration/*_test.go      真实环境集成（需要 NAZHI_USERNAME/NAZHI_PASSWORD，默认 skip）
internal/ocr/                    OCR 单元测试（含 cross-platform build tag 隔离，覆盖 Pool/cleanup/sweep/win/mu 等）
```

**SDK 单元测试**不依赖真实 ddddocr（太重），用 `recognizer` 接口注入 mock。`test/integration/` 用 build tag 隔离：
- 默认（无 tag）：单元测试，无真实凭据
- `go test -tags integration`：含真实环境集成，需 `NAZHI_USERNAME` / `NAZHI_PASSWORD`
- `go test -tags verify`：含 `verify_gitignore/` 兜底测试（CLAUDE.md 不能被 track）

## 架构深化候选（已实施 1~7，未实施 8）

| # | 候选 | 状态 | 落地位置 |
|---|---|---|---|
| #1 | Session 收口 | 已实施 | `pkg/client/session.go` sessionManager 状态机 |
| #2 | HTTP helper 私有化 | 已实施 | `request.go` 的 `httpDo` / `rawDoWithResp` 提取共享 `do()` 核心 |
| #3 | tokenparse 包 + DerefOr[T] 升包 | 已实施 | `pkg/tokenparse/tokenparse.go` + `pkg/types/deref.go` |
| #4 | sessionManager 封装 + SetBackoff race fix | 已实施 | `pkg/client/session.go` |
| #5 | `ParallelDims` 泛型 helper | 已实施 | `pkg/client/parallel.go` |
| #6 | `error_category.go` 错误分类 | 已实施 | `pkg/client/error_category.go` |
| #7 | `pkg/client/error.go` 错误码集中定义 | 未实施 | 哨兵错误集中在 `errors.go` |

> `pkg/client/parallel.go` 与 `pkg/client/error_category.go` **已实施**。
> `FetchTasks` 仍用内联 `errgroup`（业务复杂性待后续迁移），但 `ParallelDims[T]` 泛型 helper 与 `ClassifyError` 分类枚举已可复用。

## 依赖关系

```
internal/ocr  (dddocr + onnxruntime)
    ↓
pkg/client  ← Option 模式 + 公开方法 + sessionManager + errors.go (哨兵)
    ↓
pkg/tokenparse  ← SSO token 解析
    ↓
pkg/types  ← UnifiedResponse 泛型辅助 + 领域类型
    ↓
cmd/nazhi  ← cobra 命令
    ↓
test/integration  ← 真实环境 + HAR fixtures
```

## 演进方向

下一轮可能的 work（不承诺）：

1. **ParallelDims 泛型 helper** —— 把 `errgroup + appendLocked + filter(mu).merge` 模式抽出来
2. **业务 API 客户端**（如果服务端补了 `task/by-id`、`profile/update` 等）
3. **cookie jar 单飞**（如果业务场景需要多账户共享 token 池）
4. **OCR 引擎可替换** —— 当前 ddddocr 写死，未来若需要云 OCR 抽象出 Engine interface

详见 GitHub Issues 和 round-tdd 历史（`MEMORY.md` 中 `review-tdd-*` 系列）。
