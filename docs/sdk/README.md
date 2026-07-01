# SDK 参考（pkg/client、pkg/types、pkg/tokenparse）

nazhi-cli 的 Go SDK 完整开放为三个公开包，可以被任何 Go 项目 `go get` 后直接调用：

| 包 | 作用 | 文档入口 |
|---|---|---|
| [`pkg/client`](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/client) | 核心 SDK：Client 构造 + 12 个公开方法 + 10 个 Option + 15 个哨兵错误 | 本文 |
| [`pkg/types`](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/types) | 领域类型（请求/响应/任务/用户等）+ 统一响应泛型解码 | [types.go](https://github.com/Wenaixi/nazhi-cli/blob/main/pkg/types/types.go) |
| [`pkg/tokenparse`](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/tokenparse) | SSO token 从 302 Location 头 / ReturnData JSON 字节提取 | [tokenparse.go](https://github.com/Wenaixi/nazhi-cli/blob/main/pkg/tokenparse/tokenparse.go) |

设计取舍：三个包各自只做一件事，不互嵌业务逻辑。`pkg/client` 偶发性依赖 `pkg/types` 与 `pkg/tokenparse`；`pkg/types` 与 `pkg/tokenparse` 互不依赖。这样第三方库可以单独引用 `pkg/tokenparse` 不必拖进整个 SDK。

---

## 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [Client 构造与 Option 模式](#client-构造与-option-模式)
- [并发安全](#并发安全)
- [SDK 方法总览](#sdk-方法总览)
- [认证域（auth.go）](#认证域authgo)
- [Session 域（session.go）](#session-域sessiongo)
- [用户域（user.go）](#用户域usergo)
- [任务域（task.go）](#任务域taskgo)
- [自我评价域（self_eval.go）](#自我评价域self_evalgo)
- [文件域（file.go）](#文件域filego)
- [资源释放（Close）](#资源释放close)
- [错误处理](#错误处理)
- [高级用法](#高级用法)
- [pkg/tokenparse 单独使用](#pkgtokenparse-单独使用)
- [pkg/types 类型索引](#pkgtypes-类型索引)

---

## 安装

```bash
go get github.com/Wenaixi/nazhi-cli/pkg/client
go get github.com/Wenaixi/nazhi-cli/pkg/types
go get github.com/Wenaixi/nazhi-cli/pkg/tokenparse
```

Go 版本要求见仓库 `go.mod`：当前 **1.26.1**。

---

## 快速开始

最常见的脚本——登录、激活 Session、获取个人信息：

```go
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func main() {
	c, err := client.New(
		client.WithSSOBase("https://www.nazhisoft.com"),                 // 可省，默认就是这个
		client.WithBaseURL("http://139.159.205.146:8280"),              // 可省，默认就是这个
		client.WithUploadURL("http://doc.nazhisoft.com"),               // 可省，默认就是这个
		client.WithTimeout(30*time.Second),                             // 默认 15s
	)
	if err != nil {
		log.Fatalf("Client 初始化失败：%v", err)
	}
	defer c.Close() // 关闭 OCR session、释放 keep-alive 连接、清临时目录

	ctx := context.Background()

	// 1. 登录（含 OCR 验证码自动识别）
	resp, err := c.Login(ctx, types.LoginRequest{
		Username: os.Getenv("NAZHI_USERNAME"), // 学号
		Password: os.Getenv("NAZHI_PASSWORD"), // 密码
	})
	if err != nil {
		log.Fatalf("登录失败：%v", err)
	}
	token := resp.Token

	// 2. 激活业务 Session（HAR 对齐 4 步，Login 后必须调一次）
	info, err := c.ActivateSession(ctx, token)
	if err != nil {
		log.Fatalf("激活 Session 失败：%v", err)
	}
	log.Printf("已登录：%s（%s）", info.Name, info.ClassName)

	// 3. 业务操作
	tasks, err := c.FetchTasks(ctx, token)
	if err != nil {
		log.Fatalf("拉取任务失败：%v", err)
	}
	log.Printf("共 %d 个任务", len(tasks))
}
```

> ⚠️ 学号密码只从环境变量读。命令行 `-u 学号 -p 密码` 在 shell 历史里留痕，是历史版本泄露事故的根因。CI 用 secret 注入。

---

## Client 构造与 Option 模式

```go
func New(opts ...Option) (*Client, error)
```

构造函数返回 `(*Client, error)`——`error` 在 `syncCookieToken` 失败时返回（典型场景：用 `WithHTTPClient` 传的自定义 `*http.Client` 没把 `Jar` 字段设成 `*cookiejar.Jar`，导致 `X-Auth-Token` 同步不到 Cookie，后续业务接口返回空数据）。

```go
c, err := client.New(client.WithToken("xxx"))
if err != nil {
    log.Fatalf("Client 初始化失败：%v", err)
}
```

默认配置（不传 `WithHTTPClient`）下 `err` 始终为 `nil`——默认 HTTP 客户端自带 cookie jar。

### 全部 Option

| Option | 类型 | 默认 | 行为约定 / 陷阱 |
|---|---|---|---|
| `WithSSOBase(url)` | `string` | `https://www.nazhisoft.com` | 空字符串被拒绝（warn）并保留当前值；非空则赋值 |
| `WithBaseURL(url)` | `string` | `http://139.159.205.146:8280` | 同上 |
| `WithUploadURL(url)` | `string` | `http://doc.nazhisoft.com` | 同上 |
| `WithTimeout(d)` | `time.Duration` | `15s` | `d<=0` 拒绝；`c.http==nil`（已 WithHTTPClient(nil)）时拒绝；HTTP 超时含连接/TLS/响应头/响应体读取 |
| `WithHTTPClient(hc)` | `*http.Client` | 默认带 cookie jar 的客户端 | `nil` 拒绝；替换后由调用方负责 Jar；Cookie 同步假定 Jar 是 `*cookiejar.Jar` |
| `WithLogger(l)` | `*slog.Logger` | stderr WARN 级别 | `nil` 拒绝；SDK 内部 warn/debug/error 走用户注入的 handler，**不**走 cobra 通道 |
| `WithToken(t)` | `string` | 无 | 同时写 Header（`X-Auth-Token`）+ Cookie；空字符串/纯空白拒绝；延迟到 `New()` 末尾统一 `syncCookieToken`，避免 Option 顺序敏感性 |
| `WithCustomOCR(r)` | `CaptchaRecognizer` | `ocr.NewPool(0)`（含 `-tags ddddocr` 构建）/ `nil`（`!ddddocr`） | `nil` 拒绝；mock 必须实现 `Recognize([]byte) (string, error)` + `Close() error` |
| `WithOCRConcurrency(n)` | `int` | `min(4, NumCPU)`（`-tags ddddocr`）/ `0` + warn（`!ddddocr`） | `n<=0` 拒绝；预热 n 个 ONNX session，每个约 50MB，单 Login 用默认即可 |
| `WithSessionBackoff(d)` | `time.Duration` | `5s` | `d<=0` 拒绝；调整 Session 激活失败后抑制重试的冷却窗口 |

> 所有 Option 的统一约定：**非法值（`nil`/`""`/`<=0`）拒绝并 `c.logger.Warn`，保留当前值**，从不会静默覆盖。生产代码可以放心不检查 `error`。

### 顺序无关性

`WithToken` 调多少次、在 `WithSSOBase` 之前还是之后调用，结果一致——`New()` 末尾才统一执行 `syncCookieToken`，届时所有 URL 已就位。

---

## 并发安全

每个 `*Client` 实例拥有独立的 cookie jar，**天然并发安全**：

```go
c, _ := client.New(client.WithToken(token))

var wg sync.WaitGroup
for i := 0; i < 10; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 多个 goroutine 并发拉任务，c 复用，无需锁
		tasks, _ := c.FetchTasks(ctx, token)
		_ = tasks
	}()
}
wg.Wait()
```

**不能跨 Client 复用 goroutine 内部的 `*Client`**：每个 Client 的 cookie jar 互相隔离，多账户场景（一个进程跑 N 个学生）需要 `client.New()` N 次，各拿一个 `*Client`。

`Cookie jar` 的底层 `url.URL` 由 `c.baseURLParsed`（`atomic.Pointer[url.URL]`，F3 修复）保护，热路径无锁；并发写 `syncCookieToken` 之间用 CAS 解决竞争。

---

## SDK 方法总览

| 方法 | 文件 | 关键行为 | 常见错误 |
|---|---|---|---|
| `InitSession(ctx)` | auth.go | GET 登录页 → 建 JSESSIONID Cookie。Login 内部已调用，一般不直接调 | `ErrNetwork` |
| `GetSchoolID(ctx, username)` | auth.go | 学号查学校 ID，无需登录 | `ErrBusinessRejected`、`ErrInvalidPayload`（school_id 非数字） |
| `Login(ctx, req)` | auth.go | InitSession + GetSchoolID + OCR 多图多试 + validateCaptcha + validate，最后 200 JSON / 302 fallback 提 token | `ErrLoginRejected`、`ErrOCRNotConfigured`、`ErrOCRPanic`、`ErrTimeout` |
| `ActivateSession(ctx, token)` | session.go | HAR 对齐 4 步激活（`/` + 两次 `getMenu` + `getMyInfo`），DCL fast-path + backoff 缓存 | `ErrBusinessRejected`、`ErrSessionBackoff`、`ErrEmptyUserInfo` |
| `GetMyInfo(ctx, token)` | user.go | 完整 40+ 字段个人资料；先走 ActivateSession 复用步骤 4 数据避免重复 HTTP | `ErrBusinessRejected`、`ErrEmptyUserInfo`、`ErrNetwork` |
| `FetchTasks(ctx, token)` | task.go | 拉全维度任务聚合；8 路并发（errgroup.SetLimit）拉各维度 | `ErrBusinessRejected`、`ErrRetryable`（ctx cancel 触发）、`ErrEmptyUserInfo` |
| `SubmitTask(ctx, token, payload)` | task.go | 提任务，29 字段 payload 透传不裁剪 | `ErrInvalidPayload`、`ErrBusinessRejected` |
| `GetDimensions(ctx, token)` | task.go | 单独拉维度列表（CLI 未暴露，SDK 高级接口） | `ErrBusinessRejected` |
| `SubmitSelfEvaluation(ctx, token, comment)` | self_eval.go | 提交评价文本 | `ErrBusinessRejected` |
| `QuerySelfEvaluation(ctx, token)` | self_eval.go | 查评价状态 + 教师评语 | `ErrBusinessRejected`、`ErrEmptyUserInfo` |
| `QuerySelfGradEvaluation(ctx, token)` | self_eval.go | 查学期评价（SDK 高级接口） | `ErrBusinessRejected` |
| `UploadFile(ctx, filePath)` | file.go | 图片上传，自动预处理（→JPG + 压缩 ≤5MB）；**不发任何鉴权头** | `ErrNetwork`、`ErrFileTooLarge`、`ErrImageTooLarge`、`ErrUploadRejected`、`ErrRateLimited`、`ErrServiceUnavailable` |
| `Close()` | client.go | 释放 OCR session、HTTP keep-alive、临时目录；聚合 error 返回 | 多个清理错误 join 一起 |

> **没有的方法**：SDK 不暴露 `FetchTaskByID`、`UpdateProfile`、`SubmitBatchTask` 等拆 API——这些 HTTP 路径服务器未提供或未在 HAR 中验证。如有需求请开 issue。

---

## 认证域（auth.go）

### `InitSession(ctx context.Context) error`

访问 SSO 登录页，建立 `JSESSIONID` Cookie。

```go
if err := c.InitSession(ctx); err != nil { /* 网络错 */ }
```

正常情况下 `Login()` 内部会调一次，**不需要手动调**。这个方法公开是为了测试或自定义登录脚本。

### `GetSchoolID(ctx context.Context, username string) (schoolID, schoolName string, err error)`

学号查学校 ID 和名称。**无需登录**——这是一个公开 API。

```go
sid, name, err := c.GetSchoolID(ctx, "2025001")
// sid="173", name="福清一中"
```

返回错误分支：

- `errors.Is(err, ErrInvalidPayload)` — `school_id` 字段缺失或非数字（防御性检查，防止静默传给 Login）
- `errors.Is(err, ErrBusinessRejected)` — 服务端返回 code≠1
- `errors.Is(err, ErrNetwork)` — 网络层失败

### `Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error)`

完整 SSO 登录，自动处理 OCR 验证码。返回结构：

```go
type LoginRequest struct {
	SchoolID string // 可空——服务端自学号推断
	Username string
	Password string
}

type LoginResponse struct {
	Token     string         // X-Auth-Token（JWT）
	ExpiresAt time.Time      // 过期时间，绝对 time.Time
	RawData   map[string]any // 服务端 200 响应的完整 JSON 透传（`json:"-"`，不参与序列化）
}
```

**内部流程**：

```
1. InitSession（串行前置，必须最先建 JSESSIONID）
2. errgroup.WithContext 并发：
   ├─ GetSchoolID（仅当 req.SchoolID 为空）
   └─ ocrRecognizeWithRetry（最多 99 张图 × 1 次/图）
3. validateCaptcha（依赖 OCR 结果，串行）
4. POST /validate
   ├─ 200 路径 → tokenparse.ExtractFromReturnData
   └─ 302 fallback → tokenparse.ExtractFromLocation
5. buildLoginResponse → syncCookieToken（X-Auth-Token）
```

**并发优化**：步骤 2 的 `GetSchoolID` 和 `ocrRecognizeWithRetry` **无数据依赖**，通过 `errgroup.WithContext` 并发跑。`InitSession` 仍串行前置（必须最先建立 JSESSIONID），`validateCaptcha` 依赖 OCR 结果故串行。

**OCR 策略**：单图 OCR 1 次（ddddocr 对同图确定性，重试无意义），失败换新图，最多 99 张图。常量 `maxOCRAttemptsPerImage=1` + `maxOCRImagesTotal=99`。

**Token 解析**：200 与 302 两条路径都走 `pkg/tokenparse` 包，详情见 [tokenparse](#pkgtokenparse-单独使用)。

**ExpiresAt 兜底**：服务端不带 `expires_in`/`exp` 字段时默认 `now+24h`，但 c.logger.Warn 会提示「server 可能未带 expires」。

**错误分支**：

| 场景 | 错误 |
|---|---|
| `c.ocr == nil`（`!ddddocr` 构建且未注入） | `errors.Is(err, ErrOCRNotConfigured)` |
| 验证码 OCR 99 张全失败 | `errors.Is(err, ErrOCRNotConfigured)` 包装 + 原始错误 |
| 验证码/凭据错误（code≠1 / 非预期状态码） | `errors.Is(err, ErrLoginRejected)` |
| HTTP 超时 | `errors.Is(err, ErrTimeout)` |
| 网络层失败 | `errors.Is(err, ErrNetwork)` |
| OCR 识别器 panic 已被 `safeOCRRecognize` recover | `errors.Is(err, ErrOCRPanic)`，stderr 同时打 `debug.Stack()` |

**典型用法**：

```go
resp, err := c.Login(ctx, types.LoginRequest{
    Username: os.Getenv("NAZHI_USERNAME"),
    Password: os.Getenv("NAZHI_PASSWORD"),
})
if err != nil {
    if errors.Is(err, client.ErrOCRNotConfigured) {
        log.Fatal("OCR 未配置，请用预编译 release 或注入自定义识别器")
    }
    if errors.Is(err, client.ErrLoginRejected) {
        log.Fatal("学号/密码/验证码错误")
    }
    log.Fatalf("登录失败：%v", err)
}
token := resp.Token
```

---

## Session 域（session.go）

### `ActivateSession(ctx context.Context, token string) (*types.UserInfo, error)`

激活业务 Session。**HAR 抓包验证：必须按以下 4 步顺序**：

```
1. GET /                                    (建后端 Session)
2. GET /api/studentInfo/getMenu             (Referer: /homepage?token=xxx)
3. GET /api/studentInfo/getMenu             (Referer: /home)
4. GET /api/studentInfo/getMyInfo           (返回完整 UserInfo)
```

跳过任何一步都会让后续接口返回空数据。

**状态机**：v0.4.0 架构深化后由内部 `sessionManager` 状态机统一管理：

- **DCL fast-path**：同 token 第二次调用直接返回缓存 `*UserInfo`，不发起 HTTP
- **backoff 缓存**：失败后同 token 在 5 秒内重复调用返 `ErrSessionBackoff`，防止 thundering herd（可用 `WithSessionBackoff` 调窗口）
- **token 隔离**：不同 token 的失败不互相污染（4 步激活改 token 后用户信息不会留旧值）

**并发契约**：

```go
// 直接并发调本函数是安全的——sm.mu 只会序列化 4 步激活，
// 约 200-500ms 内释放
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        info, err := c.ActivateSession(ctx, token) // 安全
        _ = info
    }()
}
wg.Wait()

// 不要在外层持锁后再调本函数：sm.mu (sync.Mutex, 不可重入)
// 与外层锁形成 ABBA 死锁。
```

**返回错误**：

| 场景 | 错误 |
|---|---|
| 上次失败在 backoff 窗口内、同 token | `errors.Is(err, ErrSessionBackoff)`，含 `time.Since(lastAttempt)` |
| 4 步任一 HTTP/网络错 | `errors.Is(err, ErrNetwork)` / `ErrTimeout` / `ErrRateLimited` / `ErrServiceUnavailable` |
| 步骤 4 `getMyInfo` 业务错或返回空 | `errors.Is(err, ErrBusinessRejected)` / `ErrEmptyUserInfo` |
| ctx 取消 | `errors.Is(err, context.Canceled)` 或 `context.DeadlineExceeded`（直接 propagate，不掩盖） |

**重复调用安全**：

```go
// 同 token 第一次调：4 步 HTTP + 缓存
info1, err := c.ActivateSession(ctx, token) // ~200-500ms

// 第二次调：直接读缓存，不到 1ms
info2, err := c.ActivateSession(ctx, token) // 0 HTTP
```

---

## 用户域（user.go）

### `GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error)`

获取完整个人资料。**会触发 ActivateSession**（复用其步骤 4 的 HTTP 请求），所以同 token 第一次调会做 4 步激活，第二次调则完全复用缓存，HTTP 0 次。

字段一览（40+ 字段，详见 `pkg/types/types.go` `UserInfo`）：

- 基础身份：`Name` 姓名、`StudentNumber` 学号、`StudentID` 学生 ID、`Initials` 姓名首字母、`Pinyin` 姓名全拼
- 学校/班级：`SchoolID` 学校 ID、`SchoolName` 学校名、`GradeName` 年级、`ClassName` 班级、`Seat` 座号
- 性别/民族/证件：`GenderName`、`Nation`、`IDType`、`IDCard`
- 生日：`Birthday`（字符串版）+ `BirthdayDate`（`[y,m,d]` 数组版，自动双形态容错）
- 联系方式：`Telephone`、`Email`、`CurrentAddress`、`FamilyAddress`、`NativePlace`
- 学籍状态：`Status`、`StatusName`、`PositionName`、`YouthLeagueFlag`、`CriminalRecordFlag`
- 入学时间 + 创建/修改时间：数组 + 字符串两种形式
- 照片附件：`PhotoAttachmentID`
- 积分：`TotalPoints`、`UsedPoints`

```go
info, err := c.GetMyInfo(ctx, token)
if err != nil {
    if errors.Is(err, client.ErrEmptyUserInfo) {
        log.Println("业务成功但暂无数据")
        return
    }
    log.Fatalf("获取用户信息失败：%v", err)
}
log.Printf("欢迎 %s（%s）", info.Name, info.ClassName)
```

**返回错误**：

| 场景 | 错误 |
|---|---|
| 业务 code≠1 | `errors.Is(err, ErrBusinessRejected)` |
| 业务成功但 returnData + dataMap 都为空 | `errors.Is(err, ErrEmptyUserInfo)` |
| 网络层失败 | `errors.Is(err, ErrNetwork)` / `ErrTimeout` |

**历史**：`v0.3.4` 及更早版本曾返 `(nil, nil)` 表示空响应，CLI 输出误导性的 `null`；v0.3.5 改返 `ErrEmptyUserInfo` 让 cmd 层走统一的 status envelope。

---

## 任务域（task.go）

### `FetchTasks(ctx context.Context, token string) ([]types.Task, error)`

拉全部维度的任务。流程：`ActivateSession` → `getDimensions` → 遍历维度并发拉 `getCircleStatistics` → 聚合。

**并发控制**：`errgroup.SetLimit(min(len(dimensions), 8))`，20 维度约 3 RTT 完成。超过 50 维度考虑调整 `fetchTasksConcurrentLimit` 常量。

**部分失败语义**：

```go
tasks, err := c.FetchTasks(ctx, token)
if err != nil {
    // 重要：即便 err != nil，tasks 也可能非空！
    // 部分维度成功、部分失败的场景下，
    // FetchTasks 返回 (partialTasks, error)。
    // 不要因为 err != nil 就丢掉 tasks。
}
```

| 场景 | 返回 |
|---|---|
| 全部成功 | `(tasks, nil)` |
| 部分维度失败（业务错或网络错） | `(partialTasks, error)`，错误包装 `ErrBusinessRejected` |
| 部分维度因 ctx cancel 失败 | `(partialTasks, error)`，错误同时含 `ErrBusinessRejected` + `ErrRetryable`（F2.1 修复） |
| 全部维度因 ctx cancel 失败 | `(nil, error)`，错误只包装 `ErrRetryable`，**不**包装 `ErrBusinessRejected` |
| 全部维度业务拒绝 | `(nil, error)`，错误包装 `ErrBusinessRejected` |

**SDK 用户判定语义**：

```go
tasks, err := c.FetchTasks(ctx, token)
switch {
case err == nil:
    log.Printf("成功 %d 个任务", len(tasks))
case errors.Is(err, client.ErrRetryable):
    // 部分或全部失败因 ctx cancel——可重试
    log.Printf("中途中断（部分任务：%d）：%v，可重试", len(tasks), err)
case errors.Is(err, client.ErrBusinessRejected):
    // 部分失败含业务错误——展示 partial tasks + 服务端错误
    log.Printf("部分失败（%d 个成功）：%v", len(tasks), err)
}
```

**单维度 panic 隔离**（F10.1）：某个维度的解析 panic 会被 `fetchTasksForDimensionSafe` 的 defer recover 捕获，写入 `dimErrs`，不影响其他维度的并发拉取。

### `SubmitTask(ctx context.Context, token string, payload types.TaskSubmitPayload) (*types.TaskResult, error)`

提交任务，payload 完整透传：

```go
type TaskSubmitPayload struct {
    ID                  *int64
    Name                string
    HostName            string
    CircleDate          string
    Rank                string
    Level               string
    Content             string
    PictureList         []int64
    CircleTaskID        int64  // 必填，否则 ErrInvalidPayload
    CircleTypeID        int64  // 必填，否则 ErrInvalidPayload
    DimensionID         int64
    Hours               float64
    CircleBeginDate     string
    CircleEndDate       string
    CheckResult         string
    PatentType          string
    PatentNum           string
    Address             string
    TermName            string
    ActivityName        string
    SportsName          string
    TeamName            string
    OrgName             string
    ResultsName         string
    ObtainTime          string
    SpecialtyTechnology string
    PlayRole            string
    LikeSpecialty1      string
    LikeSpecialty2      string
    LikeSpecialty3      string
}
```

29 字段全部透传，SDK 不裁剪不处理。不同任务类型的字段差异（HAR 验证）：

| 字段 | 劳动 | 军训 | 班会 | 通用 |
|---|---|---|---|---|
| `name` | 任务原名 | `""` | `"班会"` | 任务原名 |
| `level` | `"5"` | `""` | `""` | `""` |
| `checkResult` | `""` | `"1"` | `""` | `""` |
| `address` | 学校名 | 学校名 | 班级名 | `""` |
| `orgName` | 学校名 | 学校名 | `""` | `""` |
| `playRole` | `""` | `""` | `"3"` | `"3"` |
| `hours` | `2.0` | `32.0` | `1.0` | `0.5` |

**错误分支**：

| 场景 | 错误 |
|---|---|
| `CircleTaskID==0` 或 `CircleTypeID==0` | `errors.Is(err, ErrInvalidPayload)` |
| 业务拒绝（已提交、参数错） | `errors.Is(err, ErrBusinessRejected)` |
| 网络层失败 | `errors.Is(err, ErrNetwork)` |

**重要约定**：业务错误会同时返回 `(*TaskResult, error)`——`TaskResult.Code` 与 `Code` 字段会被填充，业务方可 `errors.As(err, &bErr)` 拿到数值 code 做精细分支（`code=2` 重试 / `code=500` 致命）。

### `GetDimensions(ctx context.Context, token string) ([]types.Dimension, error)`

拉任务维度列表（思想品德、学业水平、身心健康、艺术素养、社会实践）。**SDK 高级接口**——CLI 没暴露单独命令，如需通过 SDK 自定义 UI 可用。

---

## 自我评价域（self_eval.go）

### `SubmitSelfEvaluation(ctx context.Context, token, comment string) error`

提交自我评价文本。

```go
if err := c.SubmitSelfEvaluation(ctx, token, "很好的学期"); err != nil {
    log.Fatal(err)
}
```

错误：`ErrBusinessRejected` / `ErrNetwork` / `ErrTimeout`。

### `QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error)`

查询自我评价 + 教师评语。返回结构包含 `StudentComment` / `TeacherComment` / `StudentName` / `ClassName` / `IsGrad` 等字段。

`Status` 字段 fallback 链（`returnData` → `dataMap` → `dataList[0]`），服务端任一字段格式变更都能拿到数据。

错误：`ErrBusinessRejected` / `ErrEmptyUserInfo` / `ErrNetwork`。

### `QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error)`

查询学期评价。返回泛型 map（字段不固定，SDK 不假设 schema）。**SDK 高级接口**——CLI 没暴露，如需自建 UI 用。

---

## 文件域（file.go）

### `UploadFile(ctx context.Context, filePath string) (int64, error)`

上传图片到文件服务器，返回图片 ID。

**关键设计**（v0.3.5+）：

| 行为 | 原因 |
|---|---|
| **不发任何 Token/Cookie/Authorization 头** | 文件服务器 `doc.nazhisoft.com` 是独立公共服务，发送业务 token 反而触发风控 |
| **使用独立 `newCleanClient`** | 内部 `http.Client` 无 cookie jar，杜绝 SSO Cookie 泄露 |
| **禁用 HTTP 重定向** | `CheckRedirect = noRedirect`，防止 302 跳第三方时附带请求头 |
| **共享 Transport 连接池** | 每张图 Clone 一次（O(1) struct copy + 重置 idle pool），批量上传 N 张只需 1 次 DNS+TCP+TLS 握手 |
| **上传前自动预处理** | 任意格式 → JPG + 透明合成 + 缩放/质量级联 → ≤ 5MB |
| **支持 BMP 转换提示** | stdlib 无 BMP 解码，会返回 `ErrUnsupportedFormat` 提示先用工具转 PNG/JPG |

**支持格式**：JPEG / PNG / GIF（自动取首帧）/ WEBP。BMP 需先转换。

**图片预处理**（F8.1 优化）：

```
1. sniff magic bytes（避免依赖扩展名）
2. 解码 + 透明合成到白底（flattenOnWhite）
3. jpeg.Encode(quality=92)
4. 文件 ≤ 5MB？ → 返回
5. 文件 > 2×5MB？ → 直接进缩放级联（省三次 encode）
6. jpeg.Encode(quality=80)
7. 文件 ≤ 5MB？ → 返回
8. 缩放级联（resize 不 encode，7 次 ×0.7，最后统一 encode 一次）
9. jpeg.Encode(quality=40)
10. 文件 ≤ 5MB？ → 返回
11. 兜底：缩小到极限仍超限 → ErrImageTooLarge
```

**错误分支**：

| 场景 | 错误 |
|---|---|
| 文件不存在 / 读取失败 | `errors.Is(err, ErrNetwork)` 或 `os.PathError` |
| 文件为空 | `errors.New("file is empty")` |
| 不支持的格式（含 BMP） | `errors.Is(err, ErrUnsupportedFormat)`（在 `pkg/client/image_prep.go`） |
| 压缩后仍 > 5MB | `errors.Is(err, ErrFileTooLarge)` + `ErrImageTooLarge` |
| HTTP 429 | `errors.Is(err, ErrRateLimited)` |
| HTTP 5xx | `errors.Is(err, ErrServiceUnavailable)` |
| HTTP 4xx 其他 | `errors.Is(err, ErrUploadRejected)` |
| 服务端业务拒绝（code≠1） | `errors.Is(err, ErrUploadRejected)` |
| 响应体读取失败 | `errors.Is(err, ErrNetwork)` |

**典型用法**：

```go
id, err := c.UploadFile(ctx, "./photo.jpg")
if err != nil {
    if errors.Is(err, client.ErrFileTooLarge) {
        log.Fatal("图片压缩后仍超 5MB")
    }
    if errors.Is(err, client.ErrUploadRejected) {
        log.Fatalf("上传被拒：%v", err)
    }
    log.Fatalf("上传失败：%v", err)
}
log.Printf("上传成功，图片 ID：%d", id)
```

`id` 后续可用于 `SubmitTask(..., types.TaskSubmitPayload{PictureList: []int64{id}, ...})`。

---

## 资源释放（Close）

```go
func (c *Client) Close() error
```

释放 Client 持有的资源：

- OCR session（ONNX runtime + 临时目录）
- HTTP Transport 空闲 keep-alive 连接

**约定**：业务完成后 `defer c.Close()`。Windows 上尤其重要——未调 Close 的话进程退出时 DLL 句柄未释放，`%TEMP%/nazhi-cli-ocr-*/onnxruntime.dll` 会被 `LoadLibrary` 占用到下次启动才能扫掉（v0.4.0 三轮修复的"启动清扫"是兜底）。

**多 Client 场景**：

```go
c1, _ := client.New(client.WithToken(token1))
c2, _ := client.New(client.WithToken(token2))
defer c2.Close()  // LIFO
defer c1.Close()
```

**错误聚合**：Close 可能返回多个清理错误 `errors.Join`，业务上可 `log 出来警告`，但不应阻塞退出。

---

## 错误处理

所有 SDK 错误都是 `error` 类型，可通过 `errors.Is` 精确判定：

```go
_, err := c.Login(ctx, req)
switch {
case errors.Is(err, client.ErrLoginRejected):
    // 学号/密码/验证码错误
case errors.Is(err, client.ErrOCRNotConfigured):
    // !ddddocr 构建且未注入 WithCustomOCR
case errors.Is(err, client.ErrOCRPanic):
    // OCR 识别器 panic 被 recover（极少，识别器实现 bug）
case errors.Is(err, client.ErrNetwork):
    // 网络层失败
case errors.Is(err, client.ErrTimeout):
    // ctx deadline / net/http 超时
case errors.Is(err, client.ErrRateLimited):
    // HTTP 429——退避后重试
case errors.Is(err, client.ErrServiceUnavailable):
    // HTTP 5xx——指数退避后重试
case errors.Is(err, client.ErrInvalidResponse):
    // HTTP 4xx（排除 429）——检查请求格式
case errors.Is(err, client.ErrUploadRejected):
    // 文件被服务器拒绝（仅 UploadFile）
case errors.Is(err, client.ErrFileTooLarge):
    // 图片 > 5MB（仅 UploadFile，含 ErrImageTooLarge 内嵌）
case errors.Is(err, client.ErrInvalidPayload):
    // 任务 payload 字段缺失
case errors.Is(err, client.ErrBusinessRejected):
    // 业务请求被服务端拒绝（非登录）
case errors.Is(err, client.ErrSessionBackoff):
    // session 激活在冷却窗口内
case errors.Is(err, client.ErrEmptyUserInfo):
    // 业务成功但无数据
case errors.Is(err, client.ErrRetryable):
    // ctx 取消触发，可重试（FetchTasks partial failure）
}
```

### 全部哨兵错误一览（15 个）

| 哨兵 | 触发场景 | 常用方法 |
|---|---|---|
| `ErrLoginRejected` | 登录请求被拒（code≠1 / 302 缺 token / 非预期状态码 / 验证码错） | `Login` |
| `ErrNetwork` | 底层网络失败（连接拒绝 / DNS 错 / TLS 错 / 响应体读取断流） | 所有方法 |
| `ErrTimeout` | ctx deadline 触发 / `*url.Error.Timeout()` / `net.OpError.Timeout()` | 所有方法 |
| `ErrRateLimited` | HTTP 429 | 所有方法 |
| `ErrServiceUnavailable` | HTTP 5xx | 所有方法 |
| `ErrInvalidResponse` | HTTP 4xx（排除 429） | 所有方法 |
| `ErrUploadRejected` | 上传文件域业务拒绝 / 响应 code≠1 / 4xx | `UploadFile` |
| `ErrFileTooLarge` | 图片压缩后仍 > 5MB | `UploadFile` |
| `ErrInvalidPayload` | task payload 缺字段 / GetSchoolID school_id 非数字 | `SubmitTask`、`GetSchoolID` |
| `ErrBusinessRejected` | 业务请求被服务端拒绝（非登录） | `SubmitTask` / `GetMyInfo` / `FetchTasks` / `QuerySelfEvaluation` / `QuerySelfGradEvaluation` / `GetDimensions` / `GetSchoolID` / `ActivateSession` |
| `ErrOCRNotConfigured` | `!ddddocr` 构建且未注入 OCR | `Login` |
| `ErrOCRPanic` | OCR 识别器 panic 被 `safeOCRRecognize` recover | `Login` |
| `ErrSessionBackoff` | session 激活失败后同 token 在冷却窗口内 | `ActivateSession` |
| `ErrEmptyUserInfo` | 业务成功但 `returnData` + `dataMap` 都为空 | `GetMyInfo` / `QuerySelfEvaluation` / `ActivateSession`（步骤 4 失败时） |
| `ErrRetryable` | ctx 取消导致 fetch 失败，可重试 | `FetchTasks`（partial / full） |

### 复合错误（errors.Join）

很多场景是错误链 + 包装，例如：

```go
// SessionBackoff 同时携带 lastErr 供追溯
return nil, errors.Join(
    fmt.Errorf("%w: 上次 token %q 激活失败重试 %v 前，请稍后重试或换 token",
        ErrSessionBackoff, token, time.Since(sm.lastAttempt)),
    sm.lastErr,
)

// FetchTasks 部分失败同时携带业务错 + ctx cancel 信号
return partialTasks, fmt.Errorf("%w: FetchTasks context 取消后部分维度成功: %w",
    ErrBusinessRejected,
    fmt.Errorf("%w: %w", ErrRetryable, err))
```

`errors.Is` / `errors.As` 都能穿透多跳链命中根 sentinel。

### 业务错误数值 code 精细分支

`types.CheckCode` 返回的 `*BusinessError` 保留数值 code 供精细判定：

```go
resp, err := c.GetMyInfo(ctx, token)
var bizErr *types.BusinessError
if errors.As(err, &bizErr) {
    switch bizErr.Code {
    case 2:
        // 业务约定重试
    case 401, 403:
        // token 过期或权限——重新登录
    case 500:
        // 服务端致命错
    }
}
```

---

## 高级用法

### 替换 HTTP 客户端（自定义 Transport）

```go
c, _ := client.New(
    client.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 16,
            IdleConnTimeout:     60 * time.Second,
        },
    }),
)
```

警告：用自定义 client 时必须确保 `Jar` 字段是 `*cookiejar.Jar`，否则 `WithToken` 会让 `client.New()` 返回 error 提示手动修复。

### 自定义 Logger

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
c, _ := client.New(
    client.WithLogger(logger),
    client.WithToken(token),
)
// 失败时可通过 logger.Debug 看到详细 HTTP 请求
```

SDK 内部只走 `c.logger.Warn/Debug/Error`，**不**走 cobra 或 stderr。这意味着：

- 你可以在 logger 里加 hook 把 warn 上报到 Sentry / Datadog
- 自定义 JSON handler 让日志进 ELK
- 测试中用 `slog.New(slog.NewTextHandler(io.Discard, nil))` 静默所有 SDK 日志

### 注入 Mock OCR（测试用 / CGO-free 构建）

```go
type mockOCR struct{ text string }

func (m *mockOCR) Recognize(_ []byte) (string, error) { return m.text, nil }
func (m *mockOCR) Close() error                       { return nil }

c, _ := client.New(
    client.WithCustomOCR(&mockOCR{text: "AB12"}),
)
```

**`!ddddocr` 构建场景**：

```bash
CGO_ENABLED=0 go build -o nazhi-noocr ./cmd/nazhi
```

产出的二进制 `c.ocr=nil`，`Login()` 立即返回 `ErrOCRNotConfigured`。CGO-free 消费者（如嵌入式的固定 token 场景）用 `WithCustomOCR` 注入 AI/外部识别器。

### 多账户并发

每个账户一个 `*Client`，不要跨账户复用：

```go
type account struct {
    username, password string
    c                  *client.Client
    token              string
    info               *types.UserInfo
}

func bootstrap(ctx context.Context, username, password string) (*account, error) {
    c, err := client.New()
    if err != nil { return nil, err }
    resp, err := c.Login(ctx, types.LoginRequest{
        Username: username, Password: password,
    })
    if err != nil { c.Close(); return nil, err }
    a := &account{username: username, password: password, c: c, token: resp.Token}
    a.info, err = c.ActivateSession(ctx, a.token)
    if err != nil { c.Close(); return nil, err }
    return a, nil
}

func main() {
    var wg sync.WaitGroup
    accounts := []*account{ /* ... */ }
    for _, a := range accounts {
        wg.Add(1)
        go func(a *account) {
            defer wg.Done()
            defer a.c.Close()
            // 每个 account 各自的 c 可以并发 FetchTasks
        }(a)
    }
    wg.Wait()
}
```

### 错误注入与超时控制

所有 SDK 方法都接 `ctx`：

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

tasks, err := c.FetchTasks(ctx, token)
```

`ctx.Done()` / `ctx.Err()` 会自动 propagate，不会被各方法吞掉（FetchTasks 内部做 `gctx.Err()` 检查 → 包装 `ErrRetryable` 走 errors.Is）。

---

## pkg/tokenparse 单独使用

适用：你自己写 SSO 登录脚本、不想用 `pkg/client.Login`、但要复用 token 解析逻辑。

```go
import "github.com/Wenaixi/nazhi-cli/pkg/tokenparse"

// 场景 A：从 302 Location 头提取
//
//   location := "https://www.nazhisoft.com/uiStudentLogin/login?token=eyJhbGc...&expires_in=3600"
token, expiresAt, err := tokenparse.ExtractFromLocation(location)
if err != nil { /* url.Parse 失败 */ }
if expiresAt.Before(time.Now()) {
    log.Fatal("token 已过期")
}

// 场景 B：从 ReturnData JSON 字节提取
//
//   raw := []byte(`{"code":1,"returnData":{"token":"xxx","expires_in":3600}}`)
token, expiresAt, err = tokenparse.ExtractFromReturnData(raw)
if err != nil { /* 空 body / token 类型异常 */ }
```

**过期时间解析规则**（`parseExpiresMap`）：

| 字段 | 单位 | 来源 |
|---|---|---|
| `expires_in`（优先） | 秒，相对当前时间 | SSO query |
| `exp` | Unix 秒，绝对时间 | SSO query / JWT |
| **兜底 `DefaultTokenTTL = 24h`** | 当两个字段都不存在 | `tokenparse.DefaultTokenTTL` |

畸形 URL 直接返回 `url.Parse` 底层错误（已是可读 parse error）。**注意**：`ErrLocationParseFailed` sentinel **已删除**——历史上曾定义但 `auth.go` 未用 `%w` 链入，导致 `errors.Is` 永不命中，纯死代码。

---

## pkg/types 类型索引

| 类型 | 字段说明 |
|---|---|
| `LoginRequest` | SchoolID / Username / Password |
| `LoginResponse` | Token / ExpiresAt / RawData（`json:"-"`） |
| `BusinessError` | Code（数值）/ Msg（字符串）；`errors.As(err, &b)` 精细分支 |
| `UserInfo` | 40+ 字段个人资料（详见 `pkg/types/types.go`） |
| `BirthdayDate` | Year / Month / Day；支持字符串 + 数组双形态 UnmarshalJSON |
| `Task` | 任务条目（ID、Name、Hours、Status、DimensionName 等 16 字段） |
| `TaskSubmitPayload` | 29 字段 addCircle 请求体透传 |
| `TaskResult` | Code / Msg |
| `Dimension` | ID / Name |
| `SelfEvalStatus` | 学生评语 / 教师评语 / 班级名 / 学校 ID 等 |

### pkg/types/response.go 泛型辅助

```go
// 统一响应解码
resp, err := types.DecodeResponse(bodyBytes)  // → UnifiedResponse
if err := types.CheckCode(resp); err != nil { /* code≠1 → *BusinessError */ }

// 类型安全解码（任何解码错误都返回 error，含字段缺失）
userInfo, err := types.DecodeReturnData[types.UserInfo](resp)
tasks, err := types.DecodeDataList[types.Task](resp)
selfEval, err := types.DecodeDataMap[types.SelfEvalStatus](resp)
```

- `DecodeReturnData[T]` 解析 `returnData` 字段（单个对象）
- `DecodeDataList[T]` 解析 `dataList` 字段（数组）
- `DecodeDataMap[T]` 解析 `dataMap` 字段（单对象）
- `CheckCode` 检查 `code==1`，否则返 `*BusinessError`

注：`pkg/types/deref.go` 提供 `DerefOr[T]` 安全解引用（用于 `*string` 等指针类型，转 `T` 零值兜底）。

---

## 实战错误处理骨架

```go
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/Wenaixi/nazhi-cli/pkg/types"
)

func main() {
	c, err := client.New()
	if err != nil { log.Fatal(err) }
	defer c.Close()

	ctx := context.Background()

	resp, err := c.Login(ctx, types.LoginRequest{
		Username: os.Getenv("NAZHI_USERNAME"),
		Password: os.Getenv("NAZHI_PASSWORD"),
	})
	if err != nil { handleLoginErr(err) }
	token := resp.Token

	tasks, err := c.FetchTasks(ctx, token)
	if err != nil { handleFetchErr(err) }
	_ = tasks
}

func handleLoginErr(err error) {
	switch {
	case errors.Is(err, client.ErrOCRNotConfigured):
		log.Fatal("OCR 未配置，建议下载预编译 release")
	case errors.Is(err, client.ErrLoginRejected):
		log.Fatal("学号/密码/验证码错")
	case errors.Is(err, client.ErrTimeout):
		log.Fatal("登录超时，建议调大 NAZHI_TIMEOUT")
	case errors.Is(err, client.ErrNetwork):
		var netErr *net.OpError
		if errors.As(err, &netErr) {
			log.Fatalf("网络层失败：%v", netErr)
		}
		log.Fatalf("网络失败：%v", err)
	default:
		log.Fatalf("未分类错误：%v", err)
	}
}

func handleFetchErr(err error) {
	switch {
	case errors.Is(err, client.ErrRetryable):
		log.Printf("部分完成（cancel 重试可继续）：%v", err)
	case errors.Is(err, client.ErrBusinessRejected):
		log.Printf("业务拒绝：%v", err)
	case errors.Is(err, client.ErrSessionBackoff):
		time.Sleep(5 * time.Second) // 等 backoff 窗口
	default:
		log.Printf("未知错误：%v", err)
	}
}
```

---

## 调试技巧

### 看完整 HTTP 请求 / 响应

```go
c, _ := client.New(
    client.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))),
)
```

`pkg/client/request.go` 的 `logDebug` / `logRequestHeaders` 会在 debug 级别输出：

```
→ POST https://www.nazhisoft.com/teacher/auth/studentLogin/validate
  Header: Accept: application/jso...
  Header: User-Agent: Mozilla/5.0 ...
  ← 200 (340 bytes)
```

请求头值超过 16 字符自动截断（脱敏 token 等敏感信息）。

### 看 OCR 流程

```go
c, _ := client.New(
    client.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))),
    client.WithCustomOCR(yourMock), // OCR mock 也输出日志
)

resp, err := c.Login(ctx, req)
// 日志会输出：
//   OCR 识别完成（4 字符）
//   OCR 识别成功: img=1 result_len=4
```

### 客户端构造错误排查

`client.New()` 返 error 几乎一定是 `WithHTTPClient` + `WithToken` 组合下自定义 client 的 `Jar` 不是 `*cookiejar.Jar`：

```go
jar, _ := cookiejar.New(nil)
c, err := client.New(
    client.WithHTTPClient(&http.Client{
        Jar:       jar,  // ← 必传
        Transport: http.DefaultTransport,
    }),
    client.WithToken("xxx"),
)
if err != nil { log.Fatal(err) }
```

---

## 版本契约

`pkg/client` 的所有公开方法自 v0.4.0（`internal/version/version.go`）起保持向后兼容。新增字段不会破坏现有调用方（Go 的结构体序列化容忍未知字段）。

**BREAKING 变更记录**：v0.3.1 起 `New()` 返回 `(*Client, error)`；v0.3.4 删除 7 个孤儿字段 / 0 引用的死错误；v0.4.0 session 状态机下沉到 `sessionManager`、HTTP helper 改私有名（`httpDo` / `rawDoWithResp`）、token 解析拆 `pkg/tokenparse`。

详见根目录 `CHANGELOG.md`。
