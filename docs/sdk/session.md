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
| token | GET `/` → getMenu×2 → getMyInfo，写入缓存 |

### 请求示例

```go
user, err := c.ActivateSession(ctx, token)
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
