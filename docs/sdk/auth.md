# 认证域

SSO 登录与学校信息。对应 `pkg/client/auth.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `Login` | SSO 登录（内部 OCR 验证码） | `nazhi login` |
| `InitSession` | 建立 JSESSIONID Cookie | —（Login 内部调用） |
| `GetSchoolID` | 按学号查学校信息 | —（Login 内部调用） |

## 使用方法

```go
c, err := client.New(
    client.WithSSOBase("https://www.nazhisoft.com"),
    client.WithBaseURL("http://139.159.205.146:8280"),
    client.WithTimeout(30*time.Second),
)
if err != nil { log.Fatal(err) }
defer c.Close()

resp, err := c.Login(ctx, types.LoginRequest{
    Username: "2025001",
    Password: "your-password",
})
if err != nil { log.Fatal(err) }
token := resp.Token // 后续业务接口使用
```

构建需 `-tags=ddddocr`（或 `WithCustomOCR`），否则 `Login` 返回 `ErrOCRNotConfigured`。

---

## Login

### 签名

```go
func (c *Client) Login(ctx context.Context, req types.LoginRequest) (*types.LoginResponse, error)
```

### 用户输入 vs SDK 自动

| 用户 | SDK 自动 |
|------|----------|
| `Username`（学号）、`Password` | `InitSession`（建 JSESSIONID） |
| 可选 `SchoolID` | **空则** `GetSchoolID(ctx, Username)`，用返回的 schoolId 拼登录表单 |
| — | 拉验证码图 + OCR（可多图重试 / FallbackOCR）；`LoginRequest` **无** Captcha 字段 |
| — | POST validate → 解析 JWT token / expiresAt |

**说明**：学号必须由调用方提供（或环境变量 `NAZHI_USERNAME`），SDK **不会**默认某个学号。  
自动的是：**用你填的学号去查学校 ID**（若未手填 SchoolID）。

### 请求示例

```go
// SchoolID 可省略 → SDK 按学号查学校
resp, err := c.Login(ctx, types.LoginRequest{
    Username: "2025001",
    Password: "your-password",
    // SchoolID: "123", // 可选；已有时跳过 GetSchoolID
})
```

### 响应示例

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9.xxx",
  "expiresAt": "2026-07-22T12:00:00+08:00",
  "rawData": {}
}
```

### 错误 / 注意

- `ErrLoginRejected`：凭据或验证码错误  
- `ErrOCRNotConfigured`：未启用 OCR  
- `ErrNetwork`：网络失败  

---

## InitSession / GetSchoolID

一般无需直接调用。`GetSchoolID(ctx, username)` 返回 `*types.SchoolInfo`（含 schoolId 等），供登录表单拼参。

## 相关类型

- `types.LoginRequest`：`username` / `password` / 可选 `schoolId`  
- `types.LoginResponse`：`token` / `expiresAt` / `rawData`  
- `types.SchoolInfo`：学校元数据  
- 总表：[autofill.md](./autofill.md)（空 SchoolID → 按学号查学校）  
