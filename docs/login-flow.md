# 登录流程详解

## SSO 完整流程（v0.4.0 并发版）

```
InitSession ─┐
             ├─ errgroup.WithContext ─┐
GetSchoolID ─┘                        │
                                      ├─ ValidateCaptcha ─ Login POST
ocrRecognizeWithRetry ────────────────┘                          ↓
                                                              200 JSON / 302 fallback
                                                                   ↓
                                                            syncCookieToken
                                                              (X-Auth-Token)
```

`GetSchoolID` 和 `ocrRecognizeWithRetry` **无数据依赖**，通过 `errgroup.WithContext` 并发执行；
`InitSession` 仍串行前置（必须最先建立 JSESSIONID），
`validateCaptcha` 依赖 OCR 结果故串行。

**并发优化收益**：串行版耗时 ≈ InitSession(150ms) + GetSchoolID(200ms) + OCR(2-5s) + ValidateCaptcha(150ms) + Login(300ms) ≈ 3~6s；
并发版（v0.4.0+）耗时 ≈ InitSession(150ms) + max(GetSchoolID, OCR) + ValidateCaptcha(150ms) + Login(300ms) ≈ 2.3~5.5s。

## 步骤详解

### Step 1: InitSession

```http
GET https://www.nazhisoft.com/uiStudentLogin/login
```

- **作用**：访问登录页建立 `JSESSIONID` Cookie
- **必须前置**：否则后续所有请求（验证码、登录）均无效（服务端用 JSESSIONID 跟踪 captcha 状态）
- **响应**：登录页 HTML（含验证码图片标签、JSESSIONID Set-Cookie 头）

### Step 2: GetSchoolID

```http
POST https://www.nazhisoft.com/teacher/auth/studentLogin/getSchoolIdByStudentNumber?userName={学号}
Content-Type: application/json

{"key": ""}
```

- **响应**：

```json
{
  "code": 1,
  "dataList": [
    {"school_id": "173", "NAME": "福清一中"}
  ]
}
```

- **多学校场景**：理论上 `dataList.length > 1` 时需要用户选择，SDK 当前取 `dataList[0]`，未来若需要手动选学校会扩展 `LoginRequest` 字段
- **防御性校验**：`school_id` 内部 `strconv.ParseInt` 校验，非数字返 `ErrInvalidPayload`，避免脏数据传给 validate

### Step 3: OCR 识别（最多 99 张图 × 1 次/图 = 99 次总尝试）

```go
// 多图多试策略（v0.2.1+）：
//   - 单张图片 OCR 1 次（ddddocr 对同一张图是确定性的，重试无意义）
//   - 失败则换新图，最多换 99 张
//   - 总尝试次数上限 = 1 × 99 = 99 次
//
// 内部流程：每轮先调 c.fetchCaptchaImage(ctx) 拉一张新图（atomic 计数器 seq 防缓存碰撞），
// 再走 c.safeOCRRecognize(imgBytes) OCR 识别。SDK 外部无需关心图床调用。
//
// v0.3.5+ OCR 可选构建：
//   不加 -tags ddddocr 时 c.ocr == nil，Login() 立即返回 ErrOCRNotConfigured，
//   调用方需用 WithCustomOCR 注入识别器。
//
// 此步骤与 Step 2 GetSchoolID 通过 errgroup 并发执行；
// ctx cancel 在循环顶部检测（提前 break），避免 99 张图全失败才退出。
//
// ocrRecognizeWithRetry 入口自动加 30s timeout（var ocrTimeout）
// 防止 99 张图 OCR 卡死整个 Login 调用，测试可注入更短值加速。

const (
    maxOCRAttemptsPerImage = 1  // 单图 OCR 次数（ddddocr 确定性）
    maxOCRImagesTotal      = 99 // 总尝试张数
)

for imgIdx := 0; imgIdx < maxOCRImagesTotal; imgIdx++ {
    if ctxErr := ctx.Err(); ctxErr != nil { break }  // 循环顶部 ctx 守卫
    imgBytes, err := c.fetchCaptchaImage(ctx)
    if err != nil { continue }
    text, err := c.safeOCRRecognize(imgBytes)  // defer recover 兜底 panic
    if err == nil && text != "" {
        return text, nil
    }
}
```

**OCR 失败错误**：
- `ErrOCRPanic` — 识别器 panic 被 `safeOCRRecognize` recover
- `"OCR pool is closed"` — `Pool.Close()` 后调 `Recognize`
- `"OCR is closed"` — 单个 OCR 实例已 `Close()`

### Step 4: ValidateCaptcha

```http
POST https://www.nazhisoft.com/uiStudentLogin/validateCaptcha
Content-Type: application/json

{"captcha": "AB12"}
```

- **响应**：`{"code": 1, "msg": "验证码校验成功"}`
- **关键**：服务端在 Session 中标记 `coreCheck = true`（**不是**给 captcha 校验过的"通行证"），后续 `validate` 请求 body 不带 captcha（HAR 对齐）

### Step 5: Login（200 JSON 优先 / 302 Location fallback）

```http
POST https://www.nazhisoft.com/teacher/auth/studentLogin/validate
Content-Type: application/json

{
  "schoolId": "173",
  "username": "学号",
  "password": "明文密码"   // HAR 对齐 — 不带 captcha 字段（已 validateCaptcha）
}
```

- **响应路径 1（HAR 主要命中）**：HTTP 200 JSON

```json
{
  "code": 1,
  "returnData": {
    "token": "eyJhbGciOiJIUzUxMiJ9.xxx",
    "expires_in": 1209600
  }
}
```

- **响应路径 2（fallback）**：HTTP 302 Redirect

```http
HTTP/1.1 302 Found
Location: https://www.nazhisoft.com/homepage?token=eyJhbGciOiJIUzUxMiJ9.xxx&expires_in=1209600
```

- SDK **优先处理 200 JSON**，fallback 到 302 Location
- 两条路径的 token 提取统一走 `pkg/tokenparse` 包（架构深化 #4）

#### Token 解析下沉（v0.4.0 架构深化）

```go
// 200 路径
token, expiresAt, err := tokenparse.ExtractFromReturnData(*loginResp.ReturnData)

// 302 fallback 路径
token, expiresAt, err := tokenparse.ExtractFromLocation(location)
```

两者均返回 `(token string, expiresAt time.Time, err error)` 三元组。

#### expires_in / exp 解析规则

| 字段 | 来源 | 单位 |
|---|---|---|
| `expires_in`（**优先**） | SSO query | 秒，相对当前时间 |
| `exp` | SSO query 或 JWT | Unix 秒，绝对时间 |
| **兜底 `DefaultTokenTTL = 24h`** | `tokenparse.DefaultTokenTTL` | 当两个字段都不存在 |

```go
const DefaultTokenTTL = 24 * time.Hour
```

`parseExpiresMap` 内部 helper（不被导出）：

```go
func parseExpiresMap(q map[string]any) time.Time {
    now := time.Now()
    // 优先 expires_in（相对秒数）
    if v, ok := q["expires_in"]; ok { /* strconv.Atoi + now+duration */ }
    // 次之 exp（绝对 Unix 秒）
    if v, ok := q["exp"]; ok { /* time.Unix */ }
    // 兜底 24h
    return now.Add(DefaultTokenTTL)
}
```

#### expiresAt 异常告警（v0.4.0）

`warnIfExpiresAtFallback` 检测两类异常，通过 `c.logger.Warn` 输出（走用户注入的 slog handler）：

1. **fallback 触发**：剩余寿命 > `DefaultTokenTTL - 1h`（典型 23h+）→ 触发兜底（server 没带 expires_in/exp）
2. **已过期/即将过期**：剩余寿命 < `expiresFallbackThreshold`（1h）→ server 给的 exp 已是过去时间

输出示例（`WithLogger(slog.NewJSONHandler(...))`）：

```json
{
  "level": "WARN",
  "msg": "Login token 已过期或剩余 < 1h，首次业务调用将立即 401",
  "label": "302 fallback",
  "remaining": "-2h34m",
  "expiresAt": "2026-06-29T22:30:00+08:00"
}
```

### Step 6: Token 持久化

```go
// 内部：写 Cookie 到 SSO + 业务两个域名
c.syncCookieToken(token)  // 走 c.warnSyncCookieToken，失败仅 warn 不中断
```

`syncCookieToken` 把 `baseURL` 在 `New()` 阶段预解析到 `c.baseURLParsed`（`atomic.Pointer[url.URL]`，F3 修复），
避免每次调用 `url.Parse`。直接构造 `Client{}` 绕过 `New()` 时，懒解析一次并 CAS 写回。

### LoginResponse

```go
type LoginResponse struct {
    Token     string         `json:"token"`      // JWT
    ExpiresAt time.Time      `json:"expires_at"` // 绝对时间
    RawData   map[string]any `json:"-"`          // 200 响应原始 JSON map，调试 / 字段扩展用
}
```

**历史字段清理**：
- v0.3.3 之前曾带 `RefreshAfter time.Time`（推荐刷新时间）—— 全仓 0 引用，删除
- v0.3.3 之前曾带 `UserInfo *UserInfo`（用户基本信息）—— Login 两条路径都不填充，删除

如需用户基本信息，调 `c.GetMyInfo(ctx, token)`。

## 业务 Session 激活（HAR 4 步）

```
1. GET /                              (建立后端 Session)
2. GET /api/studentInfo/getMenu        (Referer: /homepage?token=xxx)
3. GET /api/studentInfo/getMenu        (Referer: /homepage)
4. GET /api/studentInfo/getMyInfo      (返回完整 UserInfo)
```

**为什么必须 4 步**：

| 步骤 | 必要原因（HAR 推测） |
|---|---|
| `GET /` | 建后端 Session context + 设置基础 Cookie |
| `getMenu` x 2（不同 Referer） | 触发权限加载 + 菜单数据初始化 |
| `getMyInfo` | 获取完整个人资料（含座号、班级等） |

跳过任何一步都会导致后续 `task list` 等业务接口返回空数据。

### v0.4.0 sessionManager 状态机

v0.4.0 把激活逻辑收口到 `pkg/client/session.go` 的 `sessionManager` 状态机（架构深化 #1）：

```
ActivateSession(ctx, token)
  → sm.Activate
    → sm.mu.Lock
    → sm.LoadToken() == token ? (DCL fast-path)
      → 返 cachedUserInfo
    → tryActivate:
        ① ctx.Err() 检查（**先**于 backoff，避免 backoff 掩盖 ctx cancel）
        ② isBackoffHit(token) ? 
          → 返 ErrSessionBackoff（包装上一个错误）
        ③ activateFn(ctx, token) (持锁 4 步)
        ④ RecordFailure OR RecordSuccess
```

**并发安全**：
- `sm.mu` 保护所有状态变更（cookie jar 是 Client 级别共享，串行持有避免竞态写）
- `sm.token` 通过 `atomic.Value` 读外写入（fast-path 无锁读）
- `cachedUserInfo` 在 mu 临界区内写入

**backoff 设计**：
- 上次失败后 `defaultSessionBackoff = 5s`（v0.3.5+）内同 token 重复调用返 `ErrSessionBackoff`
- 防止 thundering herd（N 个 goroutine 并发激活打挂服务）
- 缓存键包含 token：不同 token 不共享冷却状态（A 登录失败不影响 B）
- 可调：`WithSessionBackoff(d)`

**与外层锁的并发契约**：

```go
// ✅ 安全模式：直接 goroutine 并发
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        c.ActivateSession(ctx, token)  // sm.mu 序列化 4 步约 200-500ms
    }()
}
wg.Wait()

// ❌ 死锁模式：外层持锁后再调
mu.Lock()
c.ActivateSession(ctx, token)  // sm.mu 不可重入，ABBA 死锁
mu.Unlock()
```

## 错误码（业务响应）

| 响应 code | 含义 | 客户端处理 |
|---|---|---|
| `1` | 登录成功 | 提取 token |
| `0` | 验证码错误 | 刷新验证码重试（SDK 内部已经自动重试 99 次） |
| `-1` | 账号或密码错误 | 终止登录（`ErrLoginRejected`） |
| `-2` | 其他错误 | 终止登录 |

## Token 格式

JWT（JSON Web Token），算法 **HS512**：

```json
{
  "sub": "学号",
  "audience": "web",
  "created": 1770382415631,  // 毫秒
  "exp": 1771592015,         // 秒（**注意**：双时间戳单位不同！）
  "userDetails": {
    "loginType": "TEACHER",
    "id": <学生ID>
  }
}
```

**关键**：JWT 的 `created` 是毫秒，`exp` 是秒——非对称单位是常见坑。
`tokenparse` 自动识别单位（`time.Unix(n, 0)` 解读为秒）。

**token 解析契约**：`pkg/tokenparse` 包暴露两个公开函数，均返回 `(string, time.Time, error)`：
- `ExtractFromLocation(location string)` — 从 302 Location 头解析
- `ExtractFromReturnData(raw json.RawMessage)` — 从 ReturnData JSON 字节解析

畸形 URL 直接返回 `url.Parse` 底层错误（已是可读 parse error）。
**注意**：`ErrLocationParseFailed` sentinel **已删除**（历史上曾定义但 `auth.go` 未用 `%w` 链入，导致 `errors.Is` 永不命中，纯死代码）。

## 完整时序图

```
SDK/CLI                       SSO (nazhisoft.com)              业务系统 (139.159.205.146:8280)
    │                                  │                                  │
    │ 1. GET /uiStudentLogin/login      │                                  │
    │──────────────────────────────────>│                                  │
    │  Set-Cookie: JSESSIONID=xxx       │                                  │
    │<──────────────────────────────────│                                  │
    │                                  │                                  │
    │ 2. POST getSchoolIdByStudentNumber│                                  │
    │    {"key":""}                     │                                  │
    │──────────────────────────────────>│                                  │
    │  {"code":1,"dataList":[{...}]}   │                                  │
    │<──────────────────────────────────│                                  │
    │                                  │                                  │
    │ 3. GET /kaptcha/kaptcha.jpg?seq=X│                                  │
    │──────────────────────────────────>│                                  │
    │  [JPEG 图片]                      │                                  │
    │<──────────────────────────────────│                                  │
    │                                  │                                  │
    │ 4. POST /validateCaptcha         │                                  │
    │    {"captcha":"AB12"}             │                                  │
    │──────────────────────────────────>│                                  │
    │  {"code":1}                       │                                  │
    │<──────────────────────────────────│                                  │
    │                                  │                                  │
    │ 5. POST /validate                  │                                  │
    │    {"schoolId":"173","username": │                                  │
    │     "x","password":"plain"}       │                                  │
    │──────────────────────────────────>│                                  │
    │  {"code":1,"returnData":{        │                                  │
    │   "token":"eyJ...","expires_in": │                                  │
    │   1209600}}                       │                                  │
    │<──────────────────────────────────│                                  │
    │                                  │                                  │
    │ 6. GET /                            │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  Set-Cookie: 业务 session                              │
    │<──────────────────────────────────────────────────────────────────────│
    │                                  │                                  │
    │ 7. GET /api/studentInfo/getMenu   │                                  │
    │    Referer: /homepage?token=xxx   │                                  │
    │    X-Auth-Token: eyJ...           │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  {"code":1,"returnData":[菜单]}    │                                  │
    │<──────────────────────────────────────────────────────────────────────│
    │                                  │                                  │
    │ 8. GET /api/studentInfo/getMenu   │                                  │
    │    Referer: /home                 │                                  │
    │    X-Auth-Token: eyJ...           │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  {"code":1,"returnData":[菜单]}    │                                  │
    │<──────────────────────────────────────────────────────────────────────│
    │                                  │                                  │
    │ 9. GET /api/studentInfo/getMyInfo  │                                  │
    │    X-Auth-Token: eyJ...           │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  {"code":1,"returnData":{User}}   │                                  │
    │<──────────────────────────────────────────────────────────────────────│
```

## 安全注意事项

⚠️ **密码明文传输**：HAR 抓包里 `hex_md5(...)` 调用被注释，密码以**原始字符串**通过 HTTPS POST 提交。
如对传输安全有要求，建议自行在外层套一层代理加密（业务网络层）。

⚠️ **登录无频率限制**：HAR 显示服务端可在短时间内接受多次登录尝试，但会因 OCR 多图多试触发本地资源开销。
SDK 单 Login 上限 ≈ 99 张图 × 30s OCR 超时 ≈ 总耗时上限约 30s（不会无限重试）。

✅ **JWT HS512 签名**：服务端持有密钥才能验证，**无法**伪造 token。

✅ **业务 token 双形态注入**：Header + Cookie 同时存在才认账，避免单 Cookie 泄露导致 SSRF 之外的攻击面。

✅ **Cookie sync 失败仅 warn**：`warnSyncCookieToken` 不抛 error，避免临时网络抖动导致整个登录失败。
但这意味着若 sync 真正失败，业务接口会立刻返回空——CI 流水线建议业务接口返回空立即触发重登录。

## 实现注意点

- **`c.ocr == nil` 立即返回** `ErrOCRNotConfigured`（避免 nil deref panic）
- **`safeOCRRecognize` defer recover**：mock 或 CGO ddddocr 边界条件下可能 panic
- **Cookie 同步在 `New()` 末尾**：避免 WithToken 顺序敏感性
- **OCR 池并发**：`Pool.Recognize` 非线程安全，`sync.Mutex` 串行化；预热 `WithOCRConcurrency(n)` 可开 n 路真并发（每实例约 50MB 内存）
- **ctx cancel 在循环顶部检查**：避免 99 张图都跑完才退出导致大量无谓 HTTP
- **`SessionBackoff` 缓存含 token**：A token 失败不影响 B token 第一次调用
- **`tryActivate` 先 ctx 后 backoff**：避免 ctx cancel 被误判为 backoff（这是 v0.4.0 关键修复之一）
