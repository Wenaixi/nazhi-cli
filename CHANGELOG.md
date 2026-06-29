# CHANGELOG

所有重要变更都会记录在此文件。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
项目遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.4.0] - 2026-06-30

v0.3.5 → v0.4.0 期间累计 **305 个 commit**，**172 文件**改动，**+12641/-6515 行**。
主要方向：review-tdd 第 15/16 轮全面修复 + 架构深化（5/8 候选）+ OCR Windows 三轮 TDD 修复。

### Features

- **架构深化（5/8 候选实施）**—
  - **#1+#5 session 收口**：`ActivateSession` 4 入口收为 1 公开 + 1 内部，删除 `activateWithBackoffCheck` / `activateSessionIfNeeded`，新增 `ensureActivated` fast-path；`sessionManager` 封装 `SetBackoff` 守卫 d≤0 + `tryActivate` 内部方法
  - **#2 HTTP helper 私有化**：`doRequest`→`httpDo`、`doRequestWithResp`→`rawDoWithResp`（私有化），公开 API 保持不变
  - **#3 DecodeUnified 原语**：`pkg/types/response.go` 新增 `DecodeUnified()` 原语（组合 `DecodeResponse` + `CheckCode`）
  - **#4 tokenparse 新包 + DerefOr[T]**：新建 `pkg/tokenparse/` 封装 token 解析（121 行 + 289 行测试），泛型 `DerefOr[T]` 升到 `pkg/types/deref.go`，auth.go 瘦身 ~40%（415→249 行）
- **OCR 启动时清扫临时目录残留**—`extractModels` 建好本进程目录后 best-effort 扫 `%TEMP%` 下别的 `nazhi-cli-ocr-*` 历史残留：能删的（已退出进程）删掉，删不掉（其他运行实例）跳过；前缀精确匹配，绝不误删其他程序目录（chrome-*, vscode-*...）。Windows 上彻底解决「login 后 %TEMP% 永久堆积」问题

### Fixed

**OCR Windows 三轮 TDD 修复**：

- **5ff0ea8** Windows DLL 占用降级—`Close` 时删 onnxruntime.dll 因 `LoadLibrary` 句柄未释放被 OS 拒绝（`Access is denied`），抽 `cleanupTempDir` helper 对 Windows 两类 errno（`ERROR_ACCESS_DENIED(5)` / `ERROR_SHARING_VIOLATION(32)`）降级返 nil。stderr 不再被权限错误污染
- **a81c9f3** GOOS=windows 守卫—上一轮注释承诺「非 Windows 永远 false」但代码不保证（Linux errno 5=EIO、32=EPIPE 也会命中），加 `goosFn` 注入点 + `runtime.GOOS == "windows"` 守卫，让降级只在 Windows 生效，避免误吞 Linux 真实错误
- **7d5dd65** 启动时清扫—见 Features 段

**review-tdd 第 15/16 轮（v0.4.0 核心修复）**：

- **session 并发与状态**：`SetBackoff` race 修复、`main panic` `closeAllClients` LIFO 顺序、`--output` 死代码删除、`Login` 并发（CallStep 改用 mutex 保护状态变量替代严格排序）
- **panic 处理**：`main.go` 顶层 panic recover 输出 `debug.Stack()` 到 stderr（之前只 `os.Exit(1)` 无任何调试信息）
- **cookie_sync 吞错**：`buildLoginResponse` 非法 JSON 后保底初始化 `RawData` 为空 map（修复 Finding #8）+ RawData 置 nil 消除二次 JSON 解析
- **image_prep 缩放级联优化**：从「缩放一次编一次」改为「resize N 次后统一 encode 一次」（Finding #11 + #12）
- **网络层**：`Transport` 加 `TLSHandshakeTimeout=10s` 防止网络挂起
- **ctx 取消传播**：`tryActivate` 先检查 `ctx.Err()` 再检查 backoff（避免被 backoff 掩盖）
- **bizURL helper 注释**：3 处裸 `baseURL` 拼接改走 `c.bizURL()` helper 集中处理
- **noRedirect 共享变量**：从 3 处 `CheckRedirect` 抽到共享 `noRedirect` 变量
- **OCR Pool `o.mu` 死锁**：`Classification` panic 时 `defer Unlock` 防死锁
- **docs 同步 + 注释中文化**：`docs/login-flow.md` 等引用已删接口清理；中文注释规范化

**review-tdd 第 15 轮**：`e5ce42f` 20 行 switch 压缩为 7 行 if-else chain；`84d4922` 编译残留 `}` 清理；`78fc5b7` URL guard 内联到 var Option；`78bd161` 3 个 1-line 同构 helper 删除

**review-tdd 第 16 轮**：`groupA` 实现空 body RawData 场景修正（`9d29915` 测试覆盖）

### Changed

- **Client 公开 API 强化**：`ActivateSession` / `FetchTasks` 等业务方法现在统一返回 `(*Client, error)`，错误路径走 `ErrBusinessRejected` / `ErrEmptyUserInfo` 哨兵
- **OCR Pool 资源管理**：v0.3.5 已有的 Pool.Close 并发安全加固 + v0.4.0 启动清扫，形成完整生命周期
- **错误哨兵扩展**：使用更频繁的 `errors.Is(err, ErrXxx)` 模式，调用方语义判断更精确

### Build

- 版本号：`0.4.0`（维护在 `internal/version/version.go`）
- 构建：所有路径显式 `-tags=ddddocr`；Makefile `build` target 仍缺 tag，**需手动** `go build -tags=ddddocr`（已知坑，CI 已正确）
- 测试：45+ 个 OCR 测试在 race + ddddocr 双 tag 下全绿；新增 `ocr_win_cleanup_test.go` / `ocr_sweep_test.go`
- 跨平台：5 平台（linux/darwin/windows × amd64/arm64）vet 验证全部通过；macOS x86_64 不支持（Microsoft 已停发）

### Compatibility

- **BREAKING**：v0.3.1 的 `client.New(...)` 改返 `(*Client, error)` 在 v0.4.0 仍是稳定的——`(*Client, error)` 签名不变
- `Login()` / `ActivateSession()` / `FetchTasks()` 等业务方法签名与 v0.3.5 完全一致
- 环境变量清单（`NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN` / `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL` / `NAZHI_TIMEOUT`）与 v0.3.5 一致
- `file upload` 子命令仍**不接受 `--token`**（v0.4.0 强化帮助文字：独立公共服务不需要业务域 token）

## [0.3.5] - 2026-06-26

### Features

- **OCR 可选构建** — 新增 `-tags ddddocr` 编译标签。不加标签时编译为纯 Go 二进制（无 CGO），`login` 命令会返回明确提示指导使用 `WithCustomOCR` 或下载预编译 release。CGO-free 嵌入式场景不再被 onnxruntime 强制依赖阻塞。
- **新增 3 个错误哨兵** — `ErrOCRNotConfigured`、`ErrEmptyUserInfo`、`ErrSessionBackoff`。SDK 用户可用 `errors.Is` 精确区分 OCR 缺失、空用户信息、session 背压三种场景。

### Fixed

- **文件上传 multipart 缺少终止边界** — 修复 upload 请求体尾部缺 `--boundary--\r\n` 导致服务端解析失败。
- **GIF 上传背景变黑** — 修复透明 GIF 合成白底时走特殊路径导致的回归。
- **图片压缩失败死循环** — 修复 JPEG 编码失败时无限重试导致 CPU 100%。
- **CLI 退出泄漏 ONNX 资源** — 修复 `os.Exit(1)` 跳过 defer 导致临时目录永久残留。
- **上传命令误读 NAZHI_TOKEN** — 修复 `file upload` 将 `NAZHI_TOKEN` 环境变量误写入 sso 域 Cookie 的问题。
- **OCR 并发关闭泄漏** — 修复池关闭后新创建的 OCR 实例不被清理、临时目录泄漏。
- **不同 token 共享 session 背压状态** — 修复 A 登录失败导致 B 也被误判为激活失败。
- **session 激活并发安全** — 修复 `ActivateSession` 无 mutex 保护导致并发请求数据污染。
- **空用户信息被当错误处理** — 修复 `GetMyInfo` 返回空数据时误报错误。
- **任务列表部分维度失败时空白** — 修复部分评价维度请求失败时整个列表不输出成功数据。
- **空消息导致日志 panic** — 修复 `resp.Msg` 为 nil 时无保护解引用。
- **维度抓取 panic 崩溃进程** — 修复某维度请求异常时整个 CLI 进程退出。
- **11 处 PII 残留** — 替换测试文件和文档中残留的真实姓名和学号。
- **HTTP 连接池限制** — 默认 `MaxIdleConnsPerHost=2` 不够用，改为 16 避免高并发反复 TLS 握手。
- **Debug 日志无谓分配内存** — 非 Debug 级别不再为日志参数做 `fmt.Sprintf` 分配。
- **Base URL 拼接不统一** — 3 处直接拼接改为 `bizURL()` helper 集中处理。
- **token flag 空字符串覆盖环境变量** — 用 `flagChanged()` 区分"没传"和"传了空值"。
- **顶层 panic 无保护** — 加 recover 统一 exit code 1，不打 stack trace。
- **Session 背压无提示** — 捕获 `ErrSessionBackoff` 时输出冷却提示等待时长。
- **context cancel 被任务抓取吞掉** — `FetchTasks` 的 goroutine 闭包检查 `gctx.Err()`。
- **文档残留已删接口引用** — 同步清理 `login-flow.md` 中已删 `GetCaptcha` 的说明。

### Changed

- **OCR 可选构建** — `pkg/client` 不再强制导入 `internal/ocr`。无 `-tags ddddocr` 时编译为纯 Go，Login 返回 `ErrOCRNotConfigured`。
- **错误哨兵体系** — 新增 4 个哨兵，覆盖 Location 解析、OCR 缺失、session 背压、空数据场景。

### Build

- 版本号：`0.3.5`
- 新增构建变体：`go build -tags ddddocr`（含 OCR）/ `无 -tags`（纯 Go 无 CGO）
- CI 增加双构建变体验证

## [0.3.4] - 2026-06-26

### Fixed

- **Token 过期时间不准** — 之前 200 路径始终用 `now+24h` 兜底，现在会解析服务端返回的 `exp`/`expires_in` 字段。
- **GetSchoolID 死分支** — 删除了一个永远不会触发的 else-if 分支（服务端只返回 NAME 字段）。
- **`derefOr` helper 简化** — nil-safe 字符串解引用，5 行变 3 行。
- **`LoginResponse.RefreshAfter` 字段删除** — 从未被服务端填充过，删掉免得误导调用方。
- **`UnifiedResponse` 6 个孤儿字段删除** — DataString、PageBean、Note、InsertID、UpdateCount、IsAttendance 全仓库 0 引用。
- **drain+close 全部统一** — 所有 HTTP 请求的 body 关闭前都会先 drain 再 close，保持 keep-alive 连接可重用。
- **5+1 处业务错误用统一哨兵包装** — `SubmitSelfEvaluation`、`QuerySelfEvaluation`、`QuerySelfGradEvaluation`、`GetMyInfo`、`fetchDimensions` 的 CheckCode 改用 `ErrBusinessRejected` 而不是之前的各种散装错误。
- **维度抓取不静默吞错误** — 之前 `fetchTasksForDimension` 遇到业务错误只 logDebug 就返回 nil，现在会返回 error 让调用方知情。
- **上传客户端 50 次握手回归** — 修复新创建的 clean client 没复用 Transport 导致批量上传反复 TLS 握手。
- **6 个 Option 加校验守卫** — `WithSSOBase`/`WithBaseURL`/`WithUploadURL`/`WithHTTPClient`/`WithOCRConcurrency`/`WithToken` 遇到空值或负值时 warn + 保留原值。
- **CLI 自动获得 Client 清理** — school 和 file_upload 改用统一 `buildClient` helper 后，自动获得 `trackClient(c)` 注册，退出时不再泄漏 ONNX 临时目录。
- **`whoami` 空数据不报错** — 当 `GetMyInfo` 返回 `(nil,nil)` 时输出 `{"status":"empty"}` 而不是裸 `null`，区分"空响应"和"激活失败"。
- **Session 激活失败背压** — 失败后缓存 + 5 秒冷却窗口，防止 N 个并发请求同时触发激活。
- **任务列表部分维度失败不吞成功数据** — 全失败返回全部错误；部分失败返回成功维度 + 错误信息；全成功正常返回。

### Changed

- **`LoginResponse.RefreshAfter` 和 `UnifiedResponse` 6 个字段删除** — BREAKING API，全仓库确认 0 引用。旧 API 响应 JSON 反序列化兼容（Go 忽略未知字段）。
- **OCR 进程级单例删除** — 不再有 `GetDefault`/`defaultOCR`/`defaultOnce`，由 Pool 替代。
- **trackInit 改用 sync.Map** — 99 次串行锁写 map 改为 `LoadOrStore`，key 已存在时 lock-free 跳过。
- **新增 `printPrompt` 函数** — 终端交互提示（如 self-eval 的"请输入评价"）走独立通道，不受 verbose 守卫，受 quiet + TTY 检测守卫。

### Build

- 版本号：`0.3.4`

## [0.3.3] - 2026-06-25

### Fixed

- **HAR 测试数据含真实姓名和学号** — `self_eval.json` 的 `student_number`/`studentName` 仍有真实信息，替换为占位值，新增自动化扫描防止再出现。
- **图片处理 69 行死代码** — `prepareImageWithStats`、`prepResult`、`PrepStats` 结构体（14 字段）、`CompressionRatio` 方法全部未用，删除后 inline 到 `prepareImageForUpload`。
- **syncCookieToken URL 解析失败静默** — 之前只有 Jar 类型断言失败会报错，URL 解析失败只打一条日志就返回 nil，现在统一返回 error。
- **SubmitTask 业务错误用了错误的错误哨兵** — 业务 code≠1 时包装成 `ErrLoginRejected`，误导 SDK 用户走重新登录流程。新增 `ErrBusinessRejected` 哨兵专门用于业务拒绝场景。
- **上传客户端污染业务连接池** — `newCleanClient` 复用业务 Client 的 Transport，调用 `CloseIdleConnections` 时会误关业务请求的 keep-alive 连接。改用 `Transport.Clone()` 创建独立 idle 连接池。

### Changed

- **`LoginResponse.UserInfo` 字段删除** — BREAKING API。登录响应从未填充过这个字段，用户信息请通过 `GetMyInfo()` 获取。

### Build

- 版本号：`0.3.3`

## [0.3.2] - 2026-06-25

### Fixed

- **集成测试编译 break** — `client.New()` 签名改 `(*Client, error)` 后集成测试没适配，CI 编译失败。
- **CLI 错误信息重复输出** — cobra 和 main 同时输出错误，终端看到两遍错误信息。统一由 `printError` 输出 JSON 格式。
- **200 登录路径缺少 token 过期告警** — 302 fallback 路径有兜底 warn，200 路径没有，不对称。
- **Referer 头里的 token 没做 URL 编码** — 虽然 JWT 是 URL-safe 的，但防御性编程应使用 `url.Values.Encode()`。
- **OCR 池并发关闭不安全** — 第二个 goroutine 关闭时第一个还在释放实例，可能重复释放同一 ONNX session。
- **任务抓取并发数不限** — 之前只留了 TODO 注释，现在加 `errgroup.SetLimit(8)` 限制并发。

### Build

- 版本号：`0.3.2`
- 新增依赖 `golang.org/x/sync v0.21.0`

## [0.3.1] - 2026-06-25

### Fixed

- **登录请求后没 drain HTTP body** — 多个 early-return 路径直接 close 连接，导致 TCP 连接无法归还 keep-alive 池，高频调用下反复建连。
- **Token 过期告警被静默** — expiresAt 兜底应打 Warn 级别，但误用了 Debug 级别，默认配置下完全看不到。
- **200 登录路径 unmarshal 失败被吞** — 错误信息只说"未找到 token"，丢了 body 内容这个关键诊断信息。
- **syncCookieToken 静默失败** — 类型断言失败只打一条 warn 就返回 nil，build client 阶段完全感知不到，后续业务接口全空时才暴露问题。改返回 error，`client.New()` 签名调整为 `(*Client, error)`。
- **OCR 重试不响应 context cancel** — 99 次循环顶部没检查 ctx，用户取消后还会跑完所有重试。
- **Session 激活并发安全** — 检查 state 后立刻放锁，4 步激活在无锁状态下执行，并发 goroutine 浪费请求且污染 cookie。
- **Session 激活第 4 步失败被掩盖** — `getMyInfoRaw` 失败只打 debug 日志，调用方收到空 UserInfo 以为激活成功。
- **WithTimeout 负数/零值没阻拦** — 0 值覆盖已有正数超时，导致请求可能永久挂起。
- **`whoami` 输出 null 被当错误处理** — `GetMyInfo` 返回 `(nil,nil)` 时走 `printError` + 退出码 1，误导用户。
- **`printError` 直接 os.Exit 绕过资源清理** — 跳过 `defer closeAllClients()`，ONNX session + 临时目录 + keep-alive 连接全部泄漏。改为标记退出码，统一在 main 末尾退出。

### Changed

- **`client.New(opts ...Option) *Client` → `(*Client, error)`** — BREAKING API。`syncCookieToken` 现在返回 error，`WithHTTPClient` 传了非 CookieJar 的 Jar 时会报错。12 个 cmd 调用点已用 `c, _ := client.New(...)` 适配。

### Build

- 版本号：`0.3.1`

## [0.3.0] - 2026-06-24

### Fixed

- **io.ReadAll 错误静默丢弃** — 网络闪断时读 body 失败，错误没说清楚，只给一句误导性的"未找到 token"。
- **验证码图片读取失败时没 drain** — 出错了也先 drain body 再 close，保证 TCP 连接可复用。
- **ExpiresAt 零值** — 200 路径的登录过期时间返回公元 0001 年，改为 `now+24h` 兜底。
- **syncCookieToken 兼容性** — 类型断言失败时输出实际类型和修复提示，方便排查。
- **Session 激活不感知 token** — 不同 token 共享同一个 session 缓存，切换 token 后可能返回旧用户数据。
- **FetchTasks 没用 session 激活** — 与其他业务方法不一致，少了 `activateSessionIfNeeded` 调用。
- **getMyInfoRaw 错误传播中断** — CheckCode 错误被截断，调用方收不到准确错误。
- **sync.Pool 裸类型断言** — 没有 `ok` 检查，GC 回收后可能 panic。
- **上传客户端零超时传播** — 父 client 没设超时时上传请求无限等待，兜底 30s。

### Changed

- **重构 request.go** — 提取 `buildRequest()` 消除 `doRequest`/`doRequestWithResp` ~40 行重复代码。
- **CLI 提取 `buildBizClient()`** — 消除 6 个命令文件各 ~15 行 env fallback + Client 构造样板，统一到 `cmd/nazhi/client_builder.go`。
- **请求日志加 debug guard** — 非 Debug 级别不再每次请求都遍历 header。
- **version 命令输出 JSON** — `nazhi version` 输出 `{"version":"0.3.0"}` 统一输出格式。

### Build

- 版本号：`0.3.0`

## [0.2.2] - 2026-06-24

### Added

- **Shell 自动补全** `nazhi completion [bash|zsh|fish|powershell]`
- **版本号子命令** `nazhi version`

### Fixed

- **Session 兜底 body 读取 bug** — `session.go:77` 中步骤 4 失败后 body 已被 defer Close 消耗的问题。

### Changed

- **文档 emoji 清理** — 全部文档和注释移除 emoji。
- **Makefile** — echo 消息纯文本化。

### Tests

- **`TestActivateSession_*` 系列** — 5 个 session fallback 测试覆盖。

### Build

- 版本号：`0.2.2`

## [0.2.1] - 2026-06-24

### Changed

- **OCR 重试策略**：`3×33` → `1×99`。同一张图 OCR 结果是确定性的，重试无意义，换图才有效。
- **Makefile echo**：移除所有 emoji，输出保持纯文本。
- **CHANGELOG / README / 文档**：全部移除 emoji，统一风格。

### Fixed

- **测试性能**：`TestPrepareImage_CompressesLargeImage` 从 3000×3000 降为 1500×1500 + Pix 直接填充（29s → 3s）。
- **CI 全平台修复**：10+ 轮修复后，5 平台（Linux amd64/arm64, macOS arm64, Windows amd64/arm64）全部构建通过。
  - Linux arm64：`gcc-aarch64-linux-gnu` 在 amd64 runner 交叉编译
  - Windows arm64：`zig cc` 在 amd64 runner 交叉编译
  - golangci-lint：`go install` 兼容 Go 1.26.1
  - softprops release：`continue-on-error: true` 处理新 release 404
- **CLAUDE.md**：OCR 并发策略、CI 修复历程、发布资产全部更新。

### Build

- 版本号：`0.2.1`

## [0.2.0] - 2026-06-22

### Features

#### 跨平台 OCR（5 平台）
- 5 平台 build tag 隔离的 `onnx_*.go` 嵌入文件（win/lin/mac × amd64/arm64）
- `ocr.GetDefault()` 进程级单例 + `sync.Mutex` 并发保护
- 99 次重试机制（同一图片）提高识别准确率
- 解压到磁盘目录供 `onnxruntime_go` 加载

#### 全自动验证码流程
- 简化 `Login()` 内部流程：InitSession → GetSchoolID → OCR → validate → 302/200 提取 token
- 优先处理 200 JSON 响应（HAR 验证），fallback 到 302 Location
- 移除所有手动/交互式验证码模式
- 自动 `syncCookieToken` 同步到 SSO + 业务域 Cookie

#### HAR 对齐的 4 步 Session 激活
- 步骤 1：GET / 初始化后端 Session
- 步骤 2：GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
- 步骤 3：GET /api/studentInfo/getMenu（Referer: /home）
- 步骤 4：GET /api/studentInfo/getMyInfo（返回完整 51 字段 UserInfo）

#### UserInfo 51 字段
- 完整暴露 `getMyInfo` 返回数据
- `birthdayStr` 字符串化（Java LocalDate JSON 数组兼容）
- 移除自定义 `Birthday` 类型

#### 图片自动压缩预处理
- 任意格式 → JPG（PNG/BMP/WEBP/GIF 支持）
- 透明合成（flattenOnWhite）
- 质量级联 → 缩放级联
- 上限 5MB
- 全部在内存中完成，不写盘

#### CLI 环境变量支持
- `NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN`
- `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL`
- `NAZHI_TIMEOUT`
- 命令行标志优先于环境变量（用 `flagChanged` 检测）
- `.env.example` 模板 + `.gitignore` 排除真实 `.env`

#### HAR 驱动集成测试
- 5 个 fixture 文件（task_flow、self_eval、military、class_meeting、labor）
- 6 个 HAR 驱动测试覆盖 FetchTasks、SubmitTask（4 种类型）、SubmitSelfEvaluation
- 真实环境 10 步端到端 `TestReal_FullChain`
- 4 个回归测试

#### 完整文档体系
- `docs/README.md` — 文档中心索引
- `docs/cli/README.md` — CLI 命令参考
- `docs/sdk/README.md` — Go SDK API 参考
- `docs/architecture.md` — 架构总览
- `docs/login-flow.md` — 登录流程详解
- `docs/cross-platform-ocr.md` — 跨平台 OCR 设计
- `docs/env-vars.md` — 环境变量参考
- `docs/har-testing.md` — HAR 驱动测试架构

### Fixes

#### Security
- **历史凭据泄露已修复**（v0.1.0 之前）：通过 `git-filter-repo` 重写所有分支和 tag 历史
- **CLI `--token` Cookie 同步**：新增 `WithToken()` Option，CLI 传 token 时同时写 Header + Cookie
- **UploadFile 禁用重定向**：cleanClient.CheckRedirect 防止 302 跳转到攻击者主机

#### Bugs
- **Task.StartDate 字段错配**：从 `startDate`（数组）改为 `startDateStr`（字符串）
- **extractTokenFromLocation URL 解析**：从 `strings.Index` 改为 `net/url.Parse`，支持 fragment
- **session.go 步骤 1/2 Body 泄漏**：defer + io.Copy 模式
- **QuerySelfGradEvaluation 错误被吞**：所有路径失败时返回明确 error
- **FetchTasks 静默失败**：用 `c.logDebug` 记录（不破坏 API）
- **output.go stderr 编码失败**：加 `fmt.Fprintln` 兜底
- **ImagePrep 兜底大小检查**：避免返回超大文件
- **stdin 无 TTY 阻塞**：`isTerminalStdin()` 检测

#### Dead Code 清理
- 删除未使用的 4 个哨兵错误（ErrTokenExpired、ErrSessionExpired、ErrIncompleteResponse、ErrUnexpectedStatus）
- 删除未使用的类型（SchoolInfo、SessionInfo）
- 删除未使用的函数（EnforceCode、自定义 min）
- 删除 debug 工具目录（cmd/debuglogin/、cmd/reallogin/、cmd/getcaptcha/、cmd/ocrtest/）

### CI/CD

- 5 平台 native runner 矩阵（ubuntu-latest、ubuntu-22.04-arm64、macos-latest、windows-latest、windows-11-arm）
- 新增 `integration` Job：tag 发布时跑真实环境集成测试（需 secrets）
- 新增 `gofmt` 检查
- 新增 `go mod tidy` 验证
- 新增 SHA256 校验和
- 二进制 `--version` 验证步骤

### Build

- Go 1.26.1
- 单二进制分发（内嵌 OCR 模型 + onnxruntime）
- Makefile：`build` / `test` / `test-verbose` / `test-integration` / `lint` / `vet` / `fmt` / `release` / `clean`

## [0.1.0] - 2026-06-21

初始发布 — nazhi-cli：纳智综合评价自动化 CLI + Go SDK。

### Features

- **SSO 全自动登录** — InitSession → GetSchoolID → 验证码处理 → Login 全流程
- **内置 OCR 验证码识别** — ddddocr 引擎 + 模型已内嵌至二进制，无需运行时下载
- **学校 ID 查询** — 根据学号获取学校信息
- **业务 Session 激活** — 登录后激活目标平台 API Session
- **用户信息查询** — 获取当前用户 profile
- **任务管理** — 列出任务 + 提交任务（支持 `@file.json` 读取）
- **自我评价** — 提交评价 & 查询评价状态
- **文件上传** — 本地图片上传至目标平台
- **跨平台构建** — Linux / macOS / Windows 三平台二进制支持

### Tech

- Go 1.26 + cobra CLI 框架
- ddddocr（ONNX Runtime）嵌入式验证码识别
- 单二进制分发，零外部依赖
