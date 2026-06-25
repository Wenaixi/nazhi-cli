# CHANGELOG

所有重要变更都会记录在此文件。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
项目遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.3.5] - 2026-06-26

六轮 review-tdd 修复 4 findings — F1 Pool.Close 后新实例泄漏 tempDir（CRITICAL）+ F2 ErrNetwork 双重包装（HIGH）+ F3 注释失实误导（LOW）+ F4 缓存设计文档化（LOW）。

### Fixed

- **ocr.go F1: Pool.Close 后 Recognize 泄漏 tempDir（CRITICAL）** — `Pool.Recognize` 不检查 `Pool.closed` 标记，`Pool.Close` 的 `closeOnce` 触发后 `sync.Pool` 仍能创建新 `*OCR` 实例，`trackInit` 将其加入 `inits` 但 `Range` 已结束，tempDir 永久泄漏。修复：`Recognize` 入口加 `Pool.closed` 检查，返回 `"OCR 池已关闭"` 错误。新增回归测试 2 个（返回错误 + inits map 无泄漏）。
- **request.go F2: doBizGet 双重重包装 ErrNetwork（HIGH）** — `doRequestWithResp` 透传 `buildRequest` 已含 `ErrNetwork` 的错误，`doBizGet` 再以 `"%w: GET %s 失败"` 包一层，导致 `errors.Is` 链中 `ErrNetwork` 出现两次。修复：`doBizGet` 透传错误不再自包装；`doRequestWithResp` 的 `c.http.Do` 错误同时统一包装 `ErrNetwork`（此前裸传）。
- **session.go F3: activateSessionIfNeeded 注释失实误导（LOW）** — 注释声称 "double-checked locking 完整模式"，但代码只有持锁单检（无锁外 fast path）。修复：改写为 "持锁单检"，阐明为何不适用 DCL（锁内模式，外层预检无意义）。
- **file.go F4: newCleanClient 缓存设计文档化（LOW）** — `cleanTransport` 经 `sync.Once` 缓存后不感知运行时 `c.http.Transport` 变更，此前无注释说明。修复：补充文档注释，阐明为 B1 缓存设计的有意取舍，非遗漏。

### Added

- `internal/ocr/ocr_pool_after_close_test.go` — F1 回归测试（Close 后 Recognize 返回错误 + inits map 无泄漏）。
- `internal/ocr/ocr.go` 注释 — `Pool.closed` 读写语义完整覆盖。

### Build

- 版本号：`0.3.5`
- 依赖不变
- 5 平台二进制跨平台构建流程不变（CI workflow 触发条件 `v*` tag）

## [0.3.4] - 2026-06-26

五轮 review-tdd 修复 18 findings — round-4（15 findings 8 worktree）+ round-5（3 deferred findings 3 worktree）。累计 5 轮 47 findings 全部解决。

### Fixed

- **auth.go G1+G2+G3: 24h magic number 3 处抽常量 + 200 路径 exp/expires_in 解析 + Login validate 错误上下文** (`auth.go`) — 3 处 `24*time.Hour` 提取为 `defaultTokenTTL`；`extractTokenFromReturnData` 新增 `parseReturnDataExpires` 解析服务端 `exp`/`expires_in` 字段（200 路径之前每次都走 now+24h 兜底 + Warn 噪声）；`Login` validate 路径 `io.ReadAll` 错误加 `status=%d read=%d bytes` 上下文。
- **auth.go M1: GetSchoolID else-if 死分支删除** — `else if school_name` 分支从未触发（HAR fixture + 真实平台只返 NAME），删除。
- **auth.go M2: stringPtrOr 重命名并简化** — 改名 `derefOr`，5 行 → 3 行。`cmp.Or` 不可行（`*nil` 解引用 panic）。
- **types.go H1: LoginResponse.RefreshAfter 死字段删除** — 与 v0.3.3 F8 删 UserInfo 同病，从未被填充。
- **response.go H2: UnifiedResponse 6 个孤儿字段删除** — DataString/PageBean/Note/InsertID/UpdateCount/IsAttendance（0 引用）。
- **request.go A: drainAndClose helper 彻底落地** — `doRequest` + `doBizGet` 内联 defer 改用 drainAndClose，F6 重构目标 5/5 全部完成。
- **self_eval.go/user.go/task.go E: 5+1 处 CheckCode 改用 ErrBusinessRejected 包装** — `SubmitSelfEvaluation`、`QuerySelfEvaluation`、`QuerySelfGradEvaluation`、`GetMyInfo`、`fetchDimensions`、`GetCircleTypeByTaskId`。关键陷阱：不能用 `*BusinessError` 作 `%w` 位，改用 `resp.Code`/`resp.Msg` 拼字符串。
- **task.go F: fetchTasksForDimension CheckCode != 1 改 return error** — 之前 logDebug + return nil 静默吞错误。
- **file.go B1: newCleanClient 50 次握手回归修复** — Client 加 `cleanTransportInit sync.Once` + `cleanTransport *http.Transport` 字段缓存克隆的 Transport。保留 F9 隔离语义：Close() 同时 `cleanTransport.CloseIdleConnections()`。
- **client.go D1: 6 个 Option 加对称守卫** — WithSSOBase/BaseURL/UploadURL（空字符串 warn+保留原值）、WithHTTPClient（nil warn）、WithOCRConcurrency（负数 warn）、WithToken（空白 warn）。
- **cmd/nazhi C1+C2+C3: buildClient 扩展 urlType 参数** — school.go + file_upload.go 改用统一 helper；C3 副作用：自动获得 `trackClient(c)`，ONNX 延迟释放问题顺带修。
- **whoami.go W1: (nil,nil) 输出状态字段** — `{"status":"empty","reason":"get_my_info_empty"}` 替代裸 `null`，三种场景可通过 JSON status 区分。
- **session.go S1: 激活失败背压 + 缓存** — 失败缓存 + 5s backoff 窗口，N 个并发 goroutine 共享 1 次尝试。
- **task.go T1: FetchTasks partial failures 聚合** — 全失败 `return nil, ErrBusinessRejected("全部 N 个维度均失败")`；部分失败 `return allTasks, fmt.Errorf("N 个维度部分失败")`；全成功 `return allTasks, nil`。

### Refactored

- **ocr.go I1: 删除 GetDefault/defaultOCR/defaultOnce 进程级单例** — 0 调用方，Pool 替代。
- **ocr.go I2: closeHook 字段删除** — 仅测试使用，改 sync.Once + Pool state 三组 invariant 等价替代。
- **ocr.go O7: trackInit 从 mutex+map 改为 sync.Map+LoadOrStore** — 99 次串行 Lock 写 map → LoadOrStore（key 已存在 lock-free 跳过）。
- **output.go L: 新增 printPrompt 函数** — quiet 静默 + non-TTY 静默，不受 verbose 守卫。self_eval_submit.go 改用。
- **cmd/nazhi: school.go + file_upload.go 去重** — inline envString/envInt/flagChanged + client.New → `buildClient(cmd, urlType, timeoutEnv)`。
- **derefOr helper 简化** — 5 行 → 3 行，nil-safe `*string` 解引用。

### Added

- **新守卫测试**：
  - `TestGetSchoolID_NoDeadBranch` — else-if 死分支不触发
  - `TestLogin_200Path_ParsesExpiresIn` — exp/expires_in 解析
  - `TestUnifiedResponse_OrphanFieldsAreNotDeserializable` — 旧 API 响应兼容性
  - `TestOption_Guardrails` — 6 个 Option 对称守卫

### Changed

- **`pkg/types.LoginResponse.RefreshAfter` 字段删除** — **BREAKING API**。v0.3.4 起 SDK 调用方不再有该字段，JSON 序列化不输出 `refresh_after`。
- **`pkg/types.UnifiedResponse` 6 个孤儿字段删除** — **BREAKING API**。DataString/PageBean/Note/InsertID/UpdateCount/IsAttendance 不再存在。全仓库 grep 确认 0 引用，旧 API 响应体仍可兼容反序列化（JSON 忽略未知字段）。

### Build

- 版本号：`0.3.4`
- 依赖不变
- 5 平台二进制跨平台构建流程不变（CI workflow 触发条件 `v*` tag）

## [0.3.3] - 2026-06-25

四轮 review-tdd 修复 7 findings — 应用新版 SKILL.md（协调者模式，主代理派 10 angle finder + 10 verifier + 4 worktree 并行 fixer agent）。

### Fixed

- **HAR fixture 真实 PII 泄露** (`test/integration/har_fixtures/self_eval.json:14,35`) — `student_number`/`studentName` 仍含真实学号 TEST2025001 与真实姓名"张三"，违反 CLAUDE.md 第 281-291 行"敏感凭据记录"条款（git-filter-repo 补救目标）。修复：替换为 `TEST2025001` / `张三` 占位值。新增 `test/integration/har_pii_redacted_test.go`（integration tag）扫描全部 5 个 fixture，含真实 PII 直接 fail。Commit `e6a235a`。
- **pkg/client/image_prep.go 69 行 dead code chain** — `prepareImageWithStats` + `prepResult` + `PrepStats` struct (14 字段) + `CompressionRatio` method 共 4 个定义合计 ~50 行只为透传一个切片，唯一 caller `prepareImageForUpload` 丢弃 Stats 字段（仅用 Data/MIME），零外部消费者。修复：删除 4 个 dead 定义 + 移除 `os.Stat` 调用，inline 核心逻辑到 `prepareImageForUpload`，外部签名 `([]byte, string, error)` 不变。Commit `6f4529e`。
- **pkg/client/auth.go syncCookieToken baseURL 解析失败静默** (`auth.go:413-417`) — F8 round1 修复时把 Jar 类型断言失败 propagate error，但 baseURL 解析失败仍 `c.logger.Warn + continue + return nil`，invariant 不对称：调用方同样无法在 build client 阶段感知失败（如畸形 URL `http://[::1` 漏右括号、`%zz` 非法转义）。修复：URL 解析失败改为 `return fmt.Errorf("syncCookieToken: 解析 base URL %q 失败: %w", raw, err)`，与 Jar 类型断言失败契约对齐。Commit `d763661`。
- **pkg/client/task.go SubmitTask 业务错误用 ErrLoginRejected 包装** (`task.go:177`) — 业务 code≠1（如 500/参数错）被 wrap 进 `ErrLoginRejected`，但 `ErrLoginRejected` 语义是"登录被拒绝（凭证无效或验证码错误）"，SDK 用户按 `docs/sdk/README.md` 推荐 `errors.Is(err, ErrLoginRejected)` 判定后错误地走重新登录流程。修复：新增 `ErrBusinessRejected = errors.New("business rejected: invalid request or server error")` 哨兵，SubmitTask 业务错误改用其包装；errors.Is(err, ErrLoginRejected) 仍专用于登录场景。Commit `8037e7b`。
- **pkg/client/file.go newCleanClient 共享 Transport 连接池污染** (`file.go:130` + `client.go:229`) — `newCleanClient` 通过 `c.http.Transport` 复用连接池（设计意图：批量上传 50 张图仅 1 次 DNS+TCP+TLS 握手），但 `Client.Close()` 的 `t.CloseIdleConnections()` 会关闭该 Transport 上**所有** idle 连接（含业务 Client 已 keep-alive 的到 sso/api 主机连接），后续业务请求强制重连 TLS。修复：`newCleanClient` 用 `(*http.Transport).Clone()` 复制独立 Transport（共享 Dialer/TLSConfig/PROXY 但 idle 池独立）；`type switch` 处理 3 种 Transport 情形（`*http.Transport` Clone / `nil` 回退 `http.DefaultTransport` / 自定义 `RoundTripper` 透传）。Commit `9e5dfc3`。

### Refactored

- **pkg/client/request.go 抽 drainAndClose helper** (`request.go` + `auth.go:133` + `file.go:72` + `session.go:72`) — 5 处 `defer func(){io.Copy(io.Discard, body); body.Close()}()` 中 3 处业务侧（Login / UploadFile / doGetMenu）verbatim 复制粘贴，注释都重复"先 drain body 再 close 让 net/http 把连接归还 keep-alive 池"。修复：在 base 层 `request.go` 加 `drainAndClose(body io.ReadCloser)` helper，3 处业务侧改用 helper；base 层 `doRequest`/`doBizGet` 内部保留内联（避免跨文件依赖）。净收益：-9 行 boilerplate + 新增 POST/streaming endpoint 不会重演 F1 修复的"漏 drain"bug 类。Commit `c8ba35f`。
- **pkg/types/types.go 删除 LoginResponse.UserInfo 死字段** (`types.go:24`) — `types.LoginResponse.UserInfo *UserInfo` 字段公开声明但 `Login()` 函数两条成功路径（200 OK / 302 Fallback）都从未填充，SDK 用户读 `resp.UserInfo` 永远拿到 nil，JSON 序列化为 `user_info:null` 误导。修复：删除字段，godoc 注明"用户基本信息请通过 `Client.GetMyInfo(ctx, token)` 获取"，收敛到 Token/ExpiresAt/RawData 三件套（实际被填充的字段）。**BREAKING API** — 见下方 Changed 段。Commit `aaf7425`。

### Added

- **新守卫测试 4 个**:
  - `test/integration/har_pii_redacted_test.go` — 扫描全部 5 个 HAR fixture 禁含 `TEST2025001` / `张三`
  - `pkg/client/sync_cookie_url_error_test.go` — 3 用例（单 URL 畸形 propagate / 双 URL 畸形短链路 / happy path 不受影响）
  - `pkg/client/login_response_no_userinfo_test.go` — JSON 序列化不再含 `"user_info"` 键
  - `pkg/client/drain_helper_test.go` — `drainAndClose` helper 单元测试
  - `pkg/client/submit_task_error_type_test.go` — httptest mock 验证 `errors.Is(err, ErrBusinessRejected) == true && errors.Is(err, ErrLoginRejected) == false`
  - `pkg/client/clean_client_test.go` — 3 组精确断言（Clone / 透传 / 回退 DefaultTransport）

### Changed

- **`pkg/types.LoginResponse.UserInfo` 字段删除** — **BREAKING API**。v0.3.3 起 SDK 调用方请改用 `Client.GetMyInfo(ctx, token)` 获取用户完整信息（`LoginResponse` 现在只包含登录 token + expires 信息）。影响范围：`pkg/types.LoginResponse` JSON 序列化输出不再含 `user_info` 键；用 `resp.UserInfo` 的旧代码会编译失败（字段不存在）。CHANGELOG v0.3.3 release note 标注。

### Build

- 版本号：`0.3.3`
- 依赖不变（仍为 `golang.org/x/sync v0.21.0`）
- 5 平台二进制跨平台构建流程不变（CI workflow 触发条件 `v*` tag）

## [0.3.2] - 2026-06-25

### Fixed

- **CI 集成测试编译 break** (`test/integration/integration_test.go` 7 处) — v0.3.1 BREAKING API `client.New() (*Client, error)` 后，`test/integration/integration_test.go` 7 处 `c := client.New(opts...)` 未适配新签名。CI workflow `.github/workflows/ci.yml:85` 必跑的 `go test -tags=integration -run=^$ ./test/integration/...` 因此 FAIL，**会阻塞所有 PR**。修复：沿用 v0.3.1 `e286134` 模式，7 处统一改为 `c, _ := client.New(...)` (或 `c2, _ :=`)。Commit `efcd09b`。
- **CLI stderr 双重输出** (`cmd/nazhi/main.go:42`) — v0.3.1 F7 修复把 `printError` 改非 `os.Exit` 后，cobra 默认行为 + main `fmt.Fprintln(os.Stderr, execErr)` 同时输出错误（cobra 写 `Error: unknown flag: --badflag` + main 写 `unknown flag: --badflag`，违反 CLAUDE.md "CLI 输出统一使用 JSON 格式"）。修复：`init()` 设 `rootCmd.SilenceErrors = true` + `SilenceUsage = true`；main:42 改 `printError(execErr)` 统一 JSON 信封。Commit `bd27332`。
- **auth.go F2 invariant 不对称** (`pkg/client/auth.go:163-169`) — `expiresAt` 兜底 warn (`time.Until(expiresAt) > 23*time.Hour`) 仅 302 fallback 路径有，200 路径 (`extractTokenFromReturnData`) 缺。`extractTokenFromReturnData` 当前总返回 `now+24h` 兜底（不解析 exp/expires_in），200 路径必走兜底却静默。修复：在 200 路径加相同 warn 守卫，与 302 路径对称。Commit `9a7f8b9`。
- **session.go step2 Referer token 未 URL 编码** (`pkg/client/session.go:36`) — `c.baseURL+"/homepage?token="+token` 直接拼接，若 token 含 `&`/`=`/`?` 等 URL 保留字符会破坏 Referer 的 query 结构。虽然 JWT 是 base64url URL-safe，defensive coding 应使用 `url.Values{"token": {token}}.Encode()`。Commit `0db0200`。
- **internal/ocr Pool.Close 并发不安全** (`internal/ocr/ocr.go:122`) — `Pool.Close()` 取 `initsMu` 拷贝 inits map 后放锁，但并发 Close 时第二个 goroutine 拿到空 map 立即 return nil，期间第一个 goroutine 仍在调 `o.Close()` 并发释放同一 OCR 实例 → 底层 CGO `onnxruntime_go` Close 未声明并发安全。修复：Pool 加 `sync.Once` 包裹 Close 闭包，确保只有一次实际 Close 逻辑跑。Commit `54ddee6`。
- **F12 FetchTasks 并发上限从 TODO 升级为真修复** (`pkg/client/task.go:50-95`) — v0.3.1 F12 仅加 6 行 TODO 注释守卫（业务维度 ≤ 20），review-tdd 二轮认为应真修。修复：引入 `errgroup.SetLimit(8)`，常量 `fetchTasksConcurrentLimit = 8`；结果聚合用 `sync.Mutex` 串行化（errgroup goroutine 间共享切片需同步）。新增 `golang.org/x/sync v0.21.0` 依赖。Commit `cd27383`。

### Refactored

- **auth.go 提取 `warnSyncCookieToken` helper** (`pkg/client/auth.go`) — 200 路径 (`auth.go:166`) 和 302 fallback 路径 (`auth.go:194`) 的 `if err := c.syncCookieToken(token); err != nil { c.logger.Warn(...) }` 完全相同，copy-paste 重复。提取 `(c *Client) warnSyncCookieToken(token, label string)` helper，两处统一调用。新增 `pkg/client/warn_sync_cookie_test.go` 直接单元测试 helper（含"日志不泄漏 token 字面值"反向断言）。Commit `1d3f04e`。
- **测试代码清理**:
  - `pkg/client/session_concurrent_test.go:166` — 删除自造 `func contains(s, substr string) bool` 函数（注释称"避免导入 strings 触发编译器对其他测试文件的额外感知"，但本包 10+ 测试文件已成功导入 strings），改用 `strings.Contains`。Commit `29b2848`。
  - `cmd/nazhi/whoami_test.go:103-129` — 内联 `os.Pipe` + goroutine `io.Copy` 模板替换为同文件已定义的 `captureStdio` helper。顺手修复了 `captureStdio` 隐藏的 drain 异步 race（goroutine drain 没同步等待，调用方读 buffer 时数据可能未到达）。Commit `7406e7f`。

### Added

- **新回归测试 6 个**:
  - `pkg/client/auth_200_expires_warn_test.go` — 验证 200 路径 `now+24h` 兜底时 WARN 级别日志输出
  - `pkg/client/warn_sync_cookie_test.go` — `warnSyncCookieToken` helper 直接单元测试（含 token 不泄漏反向断言）
  - `pkg/client/session_referer_encode_test.go` — `&` 和 `=` token 编码为 `%26` / `%3D`
  - `pkg/client/f2_strings_contains_test.go` — 锁入 `strings.Contains` 标准库语义
  - `internal/ocr/ocr_pool_close_test.go` — 8 goroutine 并发 Close 断言三组 invariant（tempDir 被删 + Pool.closed=true + Pool.inits 排空）
  - `pkg/client/task_concurrent_limit_test.go` — 20 维度场景断言 peak ≤ 8、≥ 2
  - `cmd/nazhi/main_test.go` — 验证 stderr 单一错误输出（非 cobra 默认 + main Fprintln 双重）

### Build

- 版本号：`0.3.2`
- 新增依赖：`golang.org/x/sync v0.21.0`（for `errgroup.SetLimit`）
- 5 平台二进制跨平台构建流程不变（CI workflow 触发条件 `v*` tag）

## [0.3.1] - 2026-06-25

### Added

- **二轮全仓库 code-review 流程** — 10 angles 并行扫描（line-scan/removed-behavior/cross-file/Go-pitfalls/wrapper-proxy/reuse/simplification/efficiency/altitude/conventions）+ 1-vote verify + sweep + final cut。
- **多 worktree 并行 TDD 修复** — 4 个 git worktree（auth / session+client / cmd / task）同时跑 RED→GREEN→REFACTOR→COMMIT。
- **新回归测试 11 个** — `auth_drain_test.go`、`auth_warn_test.go`、`auth_unmarshal_log_test.go`、`sync_cookie_error_test.go`、`ocr_ctx_test.go`、`session_concurrent_test.go`、`session_step4_error_test.go`、`with_timeout_test.go` 追加、`output_test.go`、`whoami_test.go`。

### Fixed

- **`auth.go:133` Login 缺 drain+close** — 6 个 early-return 路径通过 `defer httpResp.Body.Close()` 但未 drain body，net/http 强制关闭 TCP 连接无法归还 keep-alive 池。修复：与 `request.go:132-136` 一致，close 前 `io.Copy(io.Discard, httpResp.Body)`。
- **`auth.go:174` expiresAt 兜底告警降级静默** — `c.logDebug()` 输出 Warn 级别意图的告警，但默认 `slog.LevelWarn` 过滤 Debug，普通 CLI 调用完全静默，24h 后神秘失效无任何告警。修复：改回 `c.logger.Warn`。
- **`auth.go:142` 200 路径吞掉 unmarshal 错误** — `if err == nil` 守卫吞掉 unmarshal 失败，错误信息只说"未找到 token"，丢失关键诊断上下文。修复：拆 if 守卫 + `logDebug` 输出 body 摘要（与 line 191-194 非预期状态码路径处理一致）。
- **`auth.go:368` syncCookieToken 静默 warn** — 类型断言失败仅 Warn 不返回 error，`WithHTTPClient` 自定义 Jar 时业务接口返回空 dataList 但根因在 build client 阶段的 stderr Warn，跨多步调用难关联。修复：改返回 `error`，`client.New()` 签名改 `(*Client, error)` propagate。
- **`auth.go:233` ocrRecognizeWithRetry ctx 不退出** — 99 次循环顶部无 `ctx.Err()` 检查，`c.ocr.Recognize()` 是 CGO 阻塞调用不响应 ctx cancel，业务 ctx cancel 后还会跑完所有 99 次同步识别。修复：for 循环顶部加 `ctx.Err()` 检查。
- **`session.go:119` activateSessionIfNeeded TOCTOU + thundering-herd** — 经典 double-checked locking 缺陷：检查后立即放锁，4 步 ActivateSession 在无锁状态下执行。N 个并发 goroutine 触发 4N 步冗余请求 + cookie jar 污染。修复：持锁激活的完整 double-checked locking。
- **`session.go:48` ActivateSession 步骤 4 错误掩盖** — getMyInfoRaw 失败仅 logDebug，最坏情况返回仅有 Raw 的 UserInfo + nil error，调用方误判激活成功。修复：propagate error，删除步骤 3 兜底分支。
- **`client.go:72` WithTimeout nil 静默 + 0 清零** — `c.http == nil` 时静默 return；`d == 0` 仅 warn 但仍 `c.http.Timeout = 0` 覆盖任何已有正数值。修复：nil 时 warn，d=0 阻断赋值。
- **`whoami.go:31` GetMyInfo (nil,nil) 误处理为 error** — SDK 设计契约"最佳努力设计"返回 (nil,nil) 时 cmd 层调用 `printError` + `os.Exit(1)`，误导用户。修复：直接 `printJSON(info)` 输出 `null`。
- **`output.go:29,35` printError 内 os.Exit 绕过 defer Close** — `os.Exit(1)` 跳过 goroutine 栈展开，`defer closeAllClients()` 永不执行，ONNX session + 临时目录 + keep-alive 连接全部泄漏。修复：printError 标记 `pendingExitCode atomic.Int32`，main 收尾统一退出。

### Changed

- **`client.New(opts ...Option) *Client` → `(*Client, error)`** — **BREAKING API**。`error` 来自 `syncCookieToken` 失败（典型场景：`WithHTTPClient` 自定义 Jar 字段不是 `*cookiejar.Jar`）。12 个 cmd 调用点已用 `c, _ := client.New(...)` 适配；生产代码应改用 `c, err := client.New(...); if err != nil { ... }`。
- **`task.go:52` FetchTasks 并发上限** — PLAUSIBLE finding（业务系统维度数 ≤ 20 远低于 DoS 阈值），加 TODO 注释守卫说明已知设计取舍，不引入 semaphore 保持代码简洁。
- **PR 文档 README/SDK 同步更新** — `client.New` 新签名 + 错误处理示例 + Cookie jar 注意事项。

### Tests

- **F8 syncCookieToken 错误返回** — `TestNew_WithHTTPClient_NonCookieJar_ReturnsError` + 3 个 baseline。
- **F3 activateSessionIfNeeded 并发** — `session_concurrent_test.go` 同 token/不同 token 并发测试。
- **F1 Login drain** — `TestLogin_DrainsBody_On200UnexpectedEOFPath`。
- **F2 expiresAt 兜底 Warn** — `TestLogin_302Fallback_ExpiresAtFallback_LogsAtWarn`。
- **F6 200 路径 logDebug** — `TestLogin_200Path_LogsUnmarshalFailure` + `TestLogin_200Path_LogsNonJSONBody`。
- **F11 OCR ctx 退出** — `TestOCRRetry_RespectsContextCancel`。
- **F9 WithTimeout 校验** — `TestWithTimeout_ZeroDoesNotOverwriteExisting` + `TestWithTimeout_NilHTTPWarns`。
- **F10 ActivateSession 步骤 4 错误** — `TestActivateSession_Step4FailsPropagates`（重写旧的 `TestActivateSession_Step4FallsBack`）。
- **F5 whoami (nil,nil)** — `TestWhoami_GetMyInfoReturnsNil_NotTreatedAsError`。
- **F7 printError 不 os.Exit** — `TestPrintError_DoesNotCallOsExit`。
- **F7 main 退出码** — `TestMain_DeferCloseStillRuns`。

### Build

- 版本号：`0.3.1`

## [0.3.0] - 2026-06-24

### Added

- **全仓库代码审查** — 7 维度多 Agent 并行审查 + 验证 + 修复流程。

### Fixed

- **`auth.go:143` io.ReadAll 静默丢弃** — 捕获网络闪断时的读取错误，避免误导性的 "未找到 token"。
- **`auth.go:269` fetchCaptchaImage 缺 drain** — 读取出错时 drain body 再 close，保证 TCP 连接可复用。
- **`auth.go:347` ExpiresAt 零值** — 200 JSON 登录路径的 `ExpiresAt` 改为 `now+24h` 兜底，不再返回公元 0001 年。
- **`auth.go:373` syncCookieToken 兼容性** — 类型断言失败时输出实际类型和修复提示。
- **`session.go:109` token 不感知** — `activateSessionIfNeeded` 改为 token 感知守卫，不同 token 自动重新激活 4 步 session。
- **`task.go:21` 行为不一致** — `FetchTasks` 改用 `activateSessionIfNeeded`，与其他 7 个 biz 方法统一。
- **`user.go:41` 错误吞噬** — `getMyInfoRaw` 正确将 CheckCode 错误传播给调用方。
- **`ocr.go:89`+`image_prep.go:300` sync.Pool 裸断言** — 类型断言加 `ok` 检查，杜绝 GC 后 panic。
- **`file.go:121` 零超时传播** — `newCleanClient` 在父 client 无超时时兜底 30s。

### Changed

- **`request.go` 提取 `buildRequest()`** — 消除 `doRequest`/`doRequestWithResp` ~40 行重复代码。
- **CLI 提取 `buildBizClient()`** — 消除 6 个命令文件各 ~15 行重复的 env fallback + Client 构造代码，新增 `cmd/nazhi/client_builder.go`。
- **`request.go` header 日志加 debug guard** — 非 Debug 级别不再每次请求都遍历 header。
- **`Makefile` VERSION 提取** — 改用精确模式 `grep -E '^\s*var\s+Version\s*='` 避免匹配注释行。
- **`version` 命令输出 JSON** — 符合 CLI 统一输出约定，`nazhi version` 输出 `{"version":"0.3.0"}`。
- **`ocr.go` Pool 注释修正** — 说明预热只分配结构体不触发 ONNX session，GC 后可能回收。

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

优化 — 多图多试 OCR 策略优化 + 文档完善 + CI 全平台修复。

### Changed

- **OCR 重试策略**：`3×33` → `1×99`（单图 OCR 1 次，失败换新图，最多 99 张图）。ddddocr 对同一张图是确定性的，同图重试无意义，真正有效的是换图（新验证码字符集变化）。
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

**重大更新** — 跨平台 OCR + 进程级单例 + HAR 驱动测试 + Cookie 同步修复 + 完整文档体系。

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
