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

### SDK 自动（读路径）

| 步骤 | 行为 |
|------|------|
| Session 预热 | 先 `ActivateSession`；若本次激活已返回 UserInfo，**直接复用**，不再打第二遍 getMyInfo |
| 学号补学校 | 响应里已有 `studentNumber`，且 `schoolId==0` **或** `schoolName==""` 时：用**该学号**调 `GetSchoolID`（SSO 公开接口）补全 `schoolId` / `schoolName` |
| 班级名 | 按前端规则移除 `className` 中首个“级”字；不按 `gradeName` 删除前缀 |

**学号不会被 SDK 改写或伪造**；平台返回什么学号就用什么去补学校。写实提交也**不会**把学号塞进 addCircle body。

### 请求示例

```go
info, err := c.GetMyInfo(ctx, token)
// info.StudentNumber 来自平台
// info.SchoolID / SchoolName 可能经 SSO 按学号补全
```

### 响应示例

```json
{
  "id": 10001,
  "name": "张三",
  "studentNumber": "2025001",
  "schoolId": 123,
  "schoolName": "示例中学",
  "gradeId": 1,
  "gradeName": "高一",
  "classId": 8,
  "className": "高一(8)班",
  "seat": 12,
  "telephone": "13800138000",
  "genderName": "男",
  "birthdayStr": "2009-01-01",
  "familyAddress": "福建省福州市"
}
```

`className` 与前端 `userBox.vue`、`modifyBox.vue`、`header.vue` 一致，只移除首个“级”字。例如服务端返回 `高一(8)班`，因其中不含“级”字，SDK 原样返回 `高一(8)班`；CLI 仍使用原有 `className` 字段，不改变 JSON wire schema。

### 错误 / 注意

- `ErrEmptyUserInfo`：业务成功但无用户数据  
- SSO 补学校失败只打 debug 日志，不导致整个 GetMyInfo 失败  

---

## UpdateMyInfoStructured

### 签名

```go
func (c *Client) UpdateMyInfoStructured(ctx context.Context, token string, input types.UserUpdateInput) error
```

### 用户输入 vs SDK 自动

| 用户可填 | SDK 自动 / 不发 |
|----------|-----------------|
| Telephone、FamilyAddress、Hobbies、GenderName、YouthLeague、NationName、IdCardType、IDCard、BirthdayStr、Seat、StudentUuid | 中文→gender / youthLeagueFlag / nation / idType |
| Name（可选） | 映射 API key **`studentName`**（不是 `name`） |
| StudentNumber（可选高级） | 原样 `studentNumber` |
| NationalStudentNumber | **故意忽略，不写入**（前端只读全国学籍号） |

零值/空串跳过，避免覆盖服务端（**密码例外**：`StudentUuid` 始终带 key，空串表示不改密码）。

| GenderName | → gender |
|------------|----------|
| 男 | 1 |
| 女 | 2 |

| YouthLeague | → youthLeagueFlag |
|-------------|-------------------|
| 是 | 1 |
| 否 | 0 |

民族、证件类型映射见源码 `user_update.go` 的 `nationMap` / `idCardTypeMap`（汉族=1…；身份证=1…）。

### 请求示例

```go
err := c.UpdateMyInfoStructured(ctx, token, types.UserUpdateInput{
    Telephone:  "13800138000",
    GenderName: "女",
    NationName: "汉族",
    IdCardType: "中国居民身份证",
    Seat:       15,
    // 不要填 NationalStudentNumber——即使填了也不会发出
})
```

### 响应示例

成功返回 `nil`（HTTP 业务 code=1）。

### 错误 / 注意

- 不支持的中文映射值 → `ErrInvalidPayload`  
- 成功后自动 `InvalidateCachedUserInfo`，避免下次 GetMyInfo 读到更新前缓存  

---

## UpdateMyInfo / InvalidateCachedUserInfo

```go
// 原始 key（调用方自己保证 key 正确，无中文转换）
_ = c.UpdateMyInfo(ctx, token, map[string]any{"telephone": "13800138000"})
// 若绕过 Update* 改了服务端资料，可手动清缓存：
c.InvalidateCachedUserInfo()
```

`UpdateMyInfo` 成功路径也会清缓存。

## 相关类型

- `types.UserInfo`、`types.UserUpdateInput`  
- 总表：[autofill.md](./autofill.md)  
