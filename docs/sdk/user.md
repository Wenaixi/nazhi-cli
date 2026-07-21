# 用户域

查询与更新个人信息。对应 `pkg/client/user.go`、`user_update.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `GetMyInfo` | 当前用户资料 | `whoami` / `user info`（CLI 多用 JSON） |
| `UpdateMyInfo` | 裸 map 更新（API 原始 key） | — |
| `UpdateMyInfoStructured` | 友好字段 + 中文→代码 | `nazhi user update` |
| `InvalidateCachedUserInfo` | 清空 Session 缓存的 UserInfo | — |

## 使用方法

```go
info, err := c.GetMyInfo(ctx, token)
err = c.UpdateMyInfoStructured(ctx, token, types.UserUpdateInput{
    Telephone:     "13800138000",
    FamilyAddress: "福建省福州市",
    GenderName:    "男",
    YouthLeague:   "否",
})
```

---

## GetMyInfo

### 签名

```go
func (c *Client) GetMyInfo(ctx context.Context, token string) (*types.UserInfo, error)
```

### 请求示例

```go
info, err := c.GetMyInfo(ctx, token)
```

### 响应示例

```json
{
  "id": 10001,
  "name": "张三",
  "studentNumber": "2025001",
  "schoolId": 123,
  "gradeId": 1,
  "gradeName": "高一",
  "classId": 8,
  "className": "（8）班",
  "seat": 12,
  "telephone": "13800138000",
  "genderName": "男",
  "birthdayStr": "2009-01-01",
  "familyAddress": "福建省福州市"
}
```

---

## UpdateMyInfoStructured

### 签名

```go
func (c *Client) UpdateMyInfoStructured(ctx context.Context, token string, input types.UserUpdateInput) error
```

### 用户输入 vs SDK 自动

| 用户可填 | SDK 自动 / 不发 |
|----------|-----------------|
| Telephone、FamilyAddress、Hobbies、GenderName、YouthLeague、NationName、IdCardType、IDCard、BirthdayStr、Seat、StudentUuid | 中文→gender/youthLeagueFlag/nation/idType；**忽略** NationalStudentNumber（前端只读） |

零值/空串跳过（密码 `StudentUuid` 例外：空串表示不改密码，仍写入 key）。

### 请求示例

```go
err := c.UpdateMyInfoStructured(ctx, token, types.UserUpdateInput{
    Telephone:  "13800138000",
    GenderName: "女",
    NationName: "汉族",
    IdCardType: "中国居民身份证",
    Seat:       15,
})
```

### 响应示例

成功返回 `nil`（HTTP 业务 code=1）。

### 错误 / 注意

- 不支持的中文映射值 → `ErrInvalidPayload`  
- 成功后自动 `InvalidateCachedUserInfo`  

---

## UpdateMyInfo / InvalidateCachedUserInfo

```go
// 原始 key
_ = c.UpdateMyInfo(ctx, token, map[string]any{"telephone": "13800138000"})
c.InvalidateCachedUserInfo()
```

## 相关类型

- `types.UserInfo`、`types.UserUpdateInput`  
