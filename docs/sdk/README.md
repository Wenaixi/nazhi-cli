# SDK 参考 (pkg/client, pkg/types, pkg/tokenparse)

nazhi-cli 的 Go SDK 提供纳智综合评价系统的完整编程接口。三个公开包：

- `pkg/client` — 核心 SDK（Client 构造 + 全部业务方法 + Option 模式）
- `pkg/types` — 领域类型（请求/响应/任务/用户）+ 统一响应泛型解码
- `pkg/tokenparse` — SSO token 解析（Location 头 / ReturnData → (token, expiresAt)）

## 安装

```bash
go get github.com/Wenaixi/nazhi-cli/pkg/client
go get github.com/Wenaixi/nazhi-cli/pkg/types
go get github.com/Wenaixi/nazhi-cli/pkg/tokenparse
```

## 快速开始

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/Wenaixi/nazhi-cli/pkg/client"
    "github.com/Wenaixi/nazhi-cli/pkg/types"
)

func main() {
    // 创建 Client（OCR 默认启用根据构建标签：-tags ddddocr 时内嵌引擎，否则需 WithCustomOCR）
    // v0.3.1+ 返回 (*Client, error) — 错误来自 WithHTTPClient 自定义 Jar 时的 syncCookieToken 失败
    // v0.3.5+ 支持 OCR 可选构建：不加 -tags ddddocr 时 Login() 返回 ErrOCRNotConfigured
    c, _ := client.New(
        client.WithSSOBase("https://www.nazhisoft.com"),
        client.WithBaseURL("http://139.159.205.146:8280"),
        client.WithUploadURL("http://doc.nazhisoft.com"),
        client.WithTimeout(30*time.Second),
    )

    ctx := context.Background()

    // 1. 登录（学号密码从环境变量读取，不要硬编码）
    resp, err := c.Login(ctx, types.LoginRequest{
        Username: os.Getenv("NAZHI_USERNAME"),
        Password: os.Getenv("NAZHI_PASSWORD"),
    })
    if err != nil {
        log.Fatal(err)
    }
    token := resp.Token

    // 2. 激活业务 Session（HAR 对齐 4 步：/ + getMenu + getMenu + getMyInfo）
    if _, err := c.ActivateSession(ctx, token); err != nil {
        log.Fatal(err)
    }

    // 3. 业务操作
    if info, err := c.GetMyInfo(ctx, token); err == nil {
        log.Printf("欢迎 %s (%s)", info.Name, info.ClassName)
    }

    tasks, err := c.FetchTasks(ctx, token)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("共 %d 个任务", len(tasks))
}
```

## Client 构造

```go
func New(opts ...Option) (*Client, error)  // v0.3.1+ 增加 error 返回
```

`error` 在 `syncCookieToken` 失败时返回（典型场景：用 `WithHTTPClient` 传入的自定义 `*http.Client` 的 `Jar` 字段不是 `*cookiejar.Jar`，导致 `X-Auth-Token` 无法同步到 Cookie，业务接口会返回空数据）。生产代码应检查 error：

```go
c, err := client.New(client.WithToken("xxx"))
if err != nil {
    log.Fatalf("Client 初始化失败：%v", err)
}
```

或使用默认 HTTP 客户端（自带 cookie jar），不传 `WithHTTPClient` 时永不返回 error：

```go
c, _ := client.New(client.WithToken("xxx"))  // 默认配置下 err 始终为 nil
```

### Option 模式

所有 Option 都是 `var`，每个 Client 实例独立持有（赋值给 `c.xxx`）。空字符串/零值会被
拒绝并 `logger.Warn` 保留当前值，不会静默覆盖。

| Option | 说明 | 默认值 |
|--------|------|--------|
| `WithSSOBase(url)` | SSO 根地址 | `https://www.nazhisoft.com` |
| `WithBaseURL(url)` | 业务 API 根地址 | `http://139.159.205.146:8280` |
| `WithUploadURL(url)` | 文件上传服务器 | `http://doc.nazhisoft.com` |
| `WithTimeout(d)` | HTTP 超时 | 15s |
| `WithHTTPClient(hc)` | 自定义 http.Client | 默认带 cookie jar |
| `WithLogger(l)` | 自定义 slog.Logger | stderr WARN 级别 |
| `WithToken(t)` | 预置 X-Auth-Token（同时写 Header + Cookie） | 无 |
| `WithCustomOCR(r)` | 自定义 OCR（测试用或 CGO-free 构建） | 默认 ddddocr 引擎 / nil（!ddddocr） |
| `WithOCRConcurrency(n)` | 设置 OCR 并发池大小（仅 ddddocr 构建有效） | 0（懒加载单实例） |

> `WithHTTPClient` 陷阱：替换后 `syncCookieToken` 假设新 client 有 `*cookiejar.Jar`。
> 若新 client 没设 `Jar` 字段（零值 nil），`client.New()` 直接返回 error，
> 提示需要 `&http.Client{Jar: cookiejar.New(nil)}`。
> 默认配置（不传 `WithHTTPClient`）下 `err` 始终为 `nil`。

### 并发安全

每个 `Client` 实例拥有独立的 cookie jar，**天然并发安全**：

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        tasks, _ := c.FetchTasks(ctx, token)  // 安全
    }()
}
wg.Wait()
```

## SDK 方法

### 认证域 (auth.go)

#### InitSession

```go
func (c *Client) InitSession(ctx context.Context) error
```

访问 SSO 登录页建立 `JSESSIONID` Cookie。一般由 `Login()` 内部调用。

#### GetSchoolID

```go
func (c *Client) GetSchoolID(ctx context.Context, username string) (schoolID, schoolName string, err error)
```

根据学号查询学校 ID 和名称（无需登录）。返回示例：`"173", "福清一中"`。

#### Login

```go
func (c *Client) Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error)
```

完整 SSO 登录，自动处理 OCR 验证码：

```go
type LoginRequest struct {
    SchoolID string // 可空，服务端自学号推断
    Username string
    Password string
}

type LoginResponse struct {
    Token     string
    ExpiresAt time.Time
    RawData   map[string]any
}
```

内部流程：`InitSession → GetSchoolID → kaptcha.jpg → OCR 重试 → validateCaptcha → validate → 302 提取 token`。

### Session 域 (session.go)

#### ActivateSession

```go
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error)
```

激活业务 Session（**HAR 对齐 4 步**，必须按顺序否则后续接口返回空）：

1. `GET /` — 初始化后端 Session
2. `GET /api/studentInfo/getMenu`（Referer: `/homepage?token=xxx`）
3. `GET /api/studentInfo/getMenu`（Referer: `/home`）
4. `GET /api/studentInfo/getMyInfo` — 获取完整个人资料

### 任务域 (task.go)

#### FetchTasks

```go
func (c *Client) FetchTasks(ctx context.Context, token string) ([]types.Task, error)
```

拉取全部维度的任务列表。内部流程：`ActivateSession → getDimensions → 遍历 getCircleStatistics`。

#### SubmitTask

```go
func (c *Client) SubmitTask(ctx context.Context, token string, payload types.TaskSubmitPayload) (*types.TaskResult, error)
```

提交一次任务。`payload` 是完整的 29 字段 `addCircle` 请求体，**SDK 不裁剪不处理**，透传给服务器。

```go
type TaskSubmitPayload struct {
    ID                  *int64
    Name                string
    HostName            string
    CircleDate          string
    Rank                string
    Level               string
    Content             string  // AI 生成的心得体会
    PictureList         []int64 // 上传图片 ID
    CircleTaskID        int64
    CircleTypeID        int64
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

**任务类型字段差异**（HAR 验证）：

| 字段 | 劳动 | 军训 | 班会 | 通用 |
|------|------|------|------|------|
| `name` | 任务原名 | `""` | `"班会"` | 任务原名 |
| `level` | `"5"` | `""` | `""` | `""` |
| `checkResult` | `""` | `"1"` | `""` | `""` |
| `address` | 学校名 | 学校名 | 班级名 | `""` |
| `orgName` | 学校名 | 学校名 | `""` | `""` |
| `playRole` | `""` | `""` | `"3"` | `"3"` |
| `hours` | `2.0` | `32.0` | `1.0` | `0.5` |

#### GetDimensions

```go
func (c *Client) GetDimensions(ctx context.Context, token string) ([]types.Dimension, error)
```

获取任务维度列表（思想品德、学业水平、身心健康等）。

### 自我评价域 (self_eval.go)

#### SubmitSelfEvaluation

```go
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token, comment string) error
```

提交自我评价文本。

#### QuerySelfEvaluation

```go
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error)
```

查询自我评价状态 + 教师评语。

#### QuerySelfGradEvaluation

```go
func (c *Client) QuerySelfGradEvaluation(ctx context.Context, token string) (*map[string]any, error)
```

查询学期评价。

### 用户域 (user.go)

#### GetMyInfo

```go
func (c *Client) GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error)
```

获取完整个人资料（51 字段：姓名、学号、学校、年级、班级、座号、性别、出生日期等）。

### 文件域 (file.go)

#### UploadFile

```go
func (c *Client) UploadFile(ctx context.Context, filePath string) (int64, error)
```

上传图片到文件服务器，返回图片 ID。

**关键约束**：
- **不发送任何 Token/Cookie**（文件服务器独立，发送反而被风控）
- SDK 内部使用独立 `http.Client`（无 cookie jar）
- 上传前自动预处理：任意格式 → JPG + 透明合成 + 压缩至 ≤ 5MB

支持格式：JPEG、PNG、GIF（取首帧）、WEBP。BMP 需先转换。

## 错误处理

所有 SDK 错误都是 `error` 类型，可通过 `errors.Is` 判断：

```go
import "errors"

_, err := c.Login(ctx, req)
switch {
case errors.Is(err, client.ErrLoginRejected):
    // 学号/密码错误
case errors.Is(err, client.ErrOCRNotConfigured):
    // 未配置验证码识别器（!ddddocr 构建且未注入 WithCustomOCR）
case errors.Is(err, client.ErrOCRPanic):
    // OCR 识别器 panic 已被 recover（极少见，识别器实现 bug）
case errors.Is(err, client.ErrNetwork):
    // 网络问题（超时/DNS/断连）
case errors.Is(err, client.ErrFileTooLarge):
    // 图片 > 5MB
case errors.Is(err, client.ErrInvalidPayload):
    // payload 字段缺失
case errors.Is(err, client.ErrBusinessRejected):
    // 业务请求被服务端拒绝（非登录场景）
case errors.Is(err, client.ErrSessionBackoff):
    // session 激活在冷却窗口内（上次失败不久）
case errors.Is(err, client.ErrEmptyUserInfo):
    // 业务成功但无用户数据（空响应）
}
```

| 哨兵错误 | 说明 |
|---------|------|
| `ErrLoginRejected` | 登录失败（凭证错、验证码错） |
| `ErrNetwork` | 网络错误 |
| `ErrUploadRejected` | 上传被服务器拒绝 |
| `ErrFileTooLarge` | 文件 > 5MB |
| `ErrInvalidPayload` | 任务 payload 字段缺失 |
| `ErrBusinessRejected` | 业务请求被拒绝（参数错/任务已提交） |
| `ErrOCRNotConfigured` | OCR 未配置（!ddddocr 构建且未注入 WithCustomOCR） |
| `ErrOCRPanic` | OCR 识别器 panic 已被 recover |
| `ErrSessionBackoff` | session 激活在冷却窗口内 |
| `ErrEmptyUserInfo` | 业务成功但无用户数据 |

## 高级用法

### 替换 HTTP 客户端

```go
c, _ := client.New(
    client.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:    100,
            IdleConnPerHost: 10,
            IdleConnTimeout: 60 * time.Second,
        },
    }),
)
```

注意：替换后 `syncCookieToken` 假设新 client 有 `*cookiejar.Jar`。如果新 client 没设置 `Jar` 字段（零值 nil），`client.New()` 会直接返回 error，提示需要 `&http.Client{Jar: cookiejar.New(nil)}`。

### 自定义 Logger

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
c, _ := client.New(
    client.WithLogger(logger),
    client.WithToken(token),
)
失败时可通过 logger.Debug 看到详细 HTTP 请求
```

### 注入 Mock OCR（测试用 / CGO-free 构建）

```go
type mockOCR struct{ text string }
func (m *mockOCR) Recognize(_ []byte) (string, error) { return m.text, nil }

c, _ := client.New(
    client.WithCustomOCR(&mockOCR{text: "AB12"}),
)
```

> **CGO-free 场景**（不带 `-tags ddddocr` 构建）：必须使用 `WithCustomOCR` 注入识别器，
> 否则 `Login()` 返回 `ErrOCRNotConfigured`。

### 资源释放

`Client` 内部持有独立的 `*http.Client`、`sync.Pool`、OCR 临时目录等资源。
业务完成后调用 `Close()` 释放（多 goroutine 协程并发跑 OCR 后尤其需要，
否则 Windows 下 onnxruntime DLL 会临时占用 `os.TempDir`，导致清理失败）：

```go
c, _ := client.New(client.WithToken(token))
defer c.Close()
```

## token 解析 (pkg/tokenparse)

新增包。封装 SSO 登录 token 从 302 Location 头或 ReturnData JSON
字节流提取的逻辑：位置、`expires_in` / `exp` 解析、缺失时兜底 `DefaultTokenTTL = 24h`。

```go
import "github.com/Wenaixi/nazhi-cli/pkg/tokenparse"

// 场景 A：从 302 Location 头提取
//   location := "https://www.nazhisoft.com/uiStudentLogin/login?token=eyJhbGciOiJIUzI1NiJ9..."
token, exp, err := tokenparse.ExtractFromLocation(location)
if err != nil { /* url.Parse 失败 */ }

// 场景 B：从 ReturnData JSON 字节提取
//   raw := []byte(`{"code":1,"returnData":{"token":"xxx","expires_in":3600}}`)
token, exp, err = tokenparse.ExtractFromReturnData(raw)
if err != nil { /* 空 body / token 类型异常 */ }
```

## 类型定义 (pkg/types)

详见 [types.go](https://github.com/Wenaixi/nazhi-cli/blob/main/pkg/types/types.go) 源码注释，包含：

- `LoginRequest` / `LoginResponse`
- `Task` / `TaskSubmitPayload` / `TaskResult`
- `UserInfo`（51 字段）
- `SelfEvalStatus`
- `Dimension`
- `UnifiedResponse` + 泛型辅助 `DecodeReturnData[T]` / `DecodeDataList[T]` / `DecodeDataMap[T]`
