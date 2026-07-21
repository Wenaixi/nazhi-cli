# Session 域

业务 API 使用前须激活 Session（HAR 验证的 4 步）。对应 `pkg/client/session.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `ActivateSession` | 4 步激活并返回用户信息 | `nazhi session activate`（CLI 多用 `ActivateSessionJSON`） |

## 使用方法

```go
// token 来自 Login
user, err := c.ActivateSession(ctx, token)
if err != nil { log.Fatal(err) }
fmt.Println(user.Name, user.SchoolID)
```

同一 Client + 同一 token 已激活时走 DCL 快速路径；失败后有 backoff（`ErrSessionBackoff`）。

---

## ActivateSession

### 签名

```go
func (c *Client) ActivateSession(ctx context.Context, token string) (*types.UserInfo, error)
```

### 用户输入 vs SDK 自动

| 用户 | SDK 自动 |
|------|----------|
| token | 4 步 HAR：GET `/` → getMenu（homepage Referer）→ getMenu（home）→ getMyInfo |
| — | 成功写入 session 状态 + **缓存 UserInfo**（供 GetMyInfo 复用） |
| — | 同 Client + 同 token 已激活 → **DCL 快速路径**，不重跑 4 步 |
| — | 失败后 **backoff**（同 token 冷却窗口内 `ErrSessionBackoff`） |

步骤 4 的 UserInfo 会走与 GetMyInfo 相同的解析；若含学号且学校字段不全，见 [user.md](./user.md) 的 `postProcessUserInfo`（按学号补学校）。

### 请求示例

```go
user, err := c.ActivateSession(ctx, token)
// 多数业务方法内部也会按需 ActivateSession，显式调用便于尽早失败
```

### 响应示例（结构化摘要）

```json
{
  "id": 10001,
  "name": "张三",
  "studentNumber": "2025001",
  "schoolId": 123,
  "gradeName": "高一",
  "className": "（8）班",
  "seat": 12
}
```

### 错误 / 注意

- `ErrSessionBackoff`：冷却中，稍后或换 token  
- `ErrEmptyUserInfo`：业务成功但无用户数据  
- 未激活时部分业务接口可能返回空数据  

## 相关类型

- `types.UserInfo` — 见 [user.md](./user.md)  
