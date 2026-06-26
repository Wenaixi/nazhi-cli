# 登录流程详解

## SSO 完整流程

```
InitSession → GetSchoolID → OCR识别（内部含获取验证码图片 + 多图多试）→ ValidateCaptcha → Login
                                                                                   ↓
                                                                              302 → 提取 token
```

## 步骤详解

### Step 1: InitSession

```go
GET https://www.nazhisoft.com/uiStudentLogin/login
```

- **作用**：访问登录页建立 `JSESSIONID` Cookie
- **必须前置**：否则后续所有请求（验证码、登录）均无效
- **响应**：登录页 HTML（含验证码图片标签）

### Step 2: GetSchoolID

```go
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

- **多学校处理**：当 `dataList.length > 1`，需要用户手动选择（暂未实现）

### Step 3: OCR 识别（最多 99 次总尝试：1 次/图 × 99 张图）

```go
// 多图多试策略（v0.2.1+）：
//   单张图片 OCR 1 次（ddddocr 对同一张图是确定性的）
//   失败则换新图，最多换 99 张
//   总尝试次数上限 = 1 × 99 = 99 次
//
// 内部流程：每轮先调 SDK 私有方法 c.fetchCaptchaImage(ctx) 拉一张新图，
// 再走 c.ocr.Recognize(imgBytes) OCR 识别。SDK 外部无需关心图床调用。
//
// v0.3.5+ OCR 可选构建：
//   不加 -tags ddddocr 时 c.ocr == nil，Login() 立即返回 ErrOCRNotConfigured，
//   调用方需用 WithCustomOCR 注入识别器。
for imgIdx := 0; imgIdx < 99; imgIdx++ {
    imgBytes := c.fetchCaptchaImage(ctx)  // 私有方法：GET /kaptcha/kaptcha.jpg?t=<ms>
    text, err := c.ocr.Recognize(imgBytes)
    if err == nil && text != "" {
        return text, nil
    }
}
```

### Step 4: ValidateCaptcha

```go
POST https://www.nazhisoft.com/uiStudentLogin/validateCaptcha
Content-Type: application/json
{"captcha": "AB12"}
```

- **响应**：`{"code": 1, "msg": "验证码校验成功"}`
- **关键**：服务端在 Session 中标记 `coreCheck = true`，后续 validate 请求无需重复传 captcha

### Step 5: Login

```go
POST https://www.nazhisoft.com/teacher/auth/studentLogin/validate
Content-Type: application/json
{
  "schoolId": "173",
  "username": "学号",
  "password": "密码"
  // 注：HAR 对齐 — 不带 captcha 字段（已由 validateCaptcha 完成）
}
```

- **响应 (HAR 验证)**：
  - 200 JSON：`{"code": 1, "returnData": {"token": "eyJhbGc..."}}`
  - 302 Redirect：`Location: /homepage?token=eyJhbGc...`

- **SDK 优先处理 200 JSON**，fallback 到 302 Location

### Step 6: Token 持久化

```go
// SDK 内部：写 Cookie 到 SSO + 业务两个域名
c.syncCookieToken(token)
```

## 业务 Session 激活（4 步 HAR 对齐）

```
1. GET /                              (建立后端 Session)
2. GET /api/studentInfo/getMenu  (Referer: /homepage?token=xxx)
3. GET /api/studentInfo/getMenu  (Referer: /home)
4. GET /api/studentInfo/getMyInfo  (返回 UserInfo)
```

**为什么必须 4 步？**
- 业务接口对 `X-Auth-Token` 双重依赖（Header + Cookie）
- `/` 初始化后端 Session 上下文
- `getMenu` 触发权限/菜单加载
- `getMyInfo` 获取完整个人资料

跳过任何一步都会导致后续接口返回空数据。

## 错误码

| 响应 code | 含义 | 客户端处理 |
|----------|------|----------|
| 1 | 登录成功 | 提取 token |
| 0 | 验证码错误 | 刷新验证码重试 |
| -1 | 账号或密码错误 | 终止登录 |
| -2 | 其他错误 | 终止登录 |

## Token 格式

JWT (JSON Web Token), 算法 HS512：

```json
{
  "sub": "学号",
  "audience": "web",
  "created": 1770382415631,  // 毫秒
  "exp": 1771592015,         // 秒
  "userDetails": {
    "loginType": "TEACHER",
    "id": 38STUDENT_ID_REDACTED
  }
}
```

**注意**：JWT 双时间戳单位——`created` 是毫秒，`exp` 是秒。

## 完整时序图

```
浏览器/SDK                     SSO (nazhisoft.com)              业务系统 (139.159.205.146:8280)
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
    │ 3. GET /kaptcha/kaptcha.jpg?t=xxx│                                  │
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
    │    {"schoolId":"","username":"x", │                                  │
    │     "password":"plain"}           │                                  │
    │──────────────────────────────────>│                                  │
    │  {"code":1,"returnData":{        │                                  │
    │   "token":"eyJ..."}}              │                                  │
    │<──────────────────────────────────│                                  │
    │                                  │                                  │
    │ 6. GET /homepage?token=xxx        │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  Set-Cookie: X-Auth-Token         │                                  │
    │<──────────────────────────────────────────────────────────────────────│
    │                                  │                                  │
    │ 7. GET /api/studentInfo/getMenu  │                                  │
    │    X-Auth-Token: eyJ...          │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  {"code":1,"returnData":[菜单]}   │                                  │
    │<──────────────────────────────────────────────────────────────────────│
    │                                  │                                  │
    │ 8. GET /api/studentInfo/getMyInfo│                                  │
    │    X-Auth-Token: eyJ...          │                                  │
    │──────────────────────────────────────────────────────────────────────>│
    │  {"code":1,"returnData":{User}}  │                                  │
    │<──────────────────────────────────────────────────────────────────────│
```

## 安全注意事项

⚠️ **密码明文传输**：JS 源码中 `hex_md5()` 调用被注释，密码以原始字符串通过 HTTPS POST 提交。

⚠️ **登录无频率限制**：HAR 显示可在短时间内多次请求。

✅ **JWT HS512 签名**：服务端必须持有密钥才能验证（无法伪造）。
