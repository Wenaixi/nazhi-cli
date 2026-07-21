# 迁移指南：v1.x → v2.0.0（深度原生修复）

## 为什么做这次破坏性更新

v1.3.0 之前的 SDK 对服务端返回的数据做了大量"主动处理"：

| 处理类型 | SDK 行为 | 问题 |
|---------|---------|------|
| 时间格式 | 字符串 → `time.Time` / `DateOnly` → RFC3339 | 原始 `"2026-01-12"` 变成了 `"2026-01-12T00:00:00+08:00"` |
| 字段命名 | snake_case → camelCase | 服务端返回 `circle_task_name`，SDK 输出 `circleTaskName` |
| 状态值 | int/string → bool | `status: 2`（被撤回）变成了 `approved: false`，丢失原始状态 |
| 数据后处理 | 学校名降级、班级名前缀清理 | SDK 偷偷调用外部 API 修改用户数据 |

v2.0.0 的核心原则是：**SDK 不做任何数据转换，返回与服务端完全一致的原始数据**。

---

## 核心变更一览

| 变更 | 说明 | 影响程度 |
|------|------|---------|
| 所有时间字段改为 `string` | 不再解析为 `time.Time`，保留服务端原始字符串 | 🔴 破坏性 |
| 所有 JSON tag 改为 snake_case | 与服务端字段名完全一致 | 🔴 破坏性 |
| 保留原始状态值 | 不再把 int/string 转 bool，保留原始值 | 🔴 破坏性 |
| 删除 `postProcessUserInfo` | 不再修改 ClassName/SchoolName | 🟡 行为变化 |
| CircleImage tag 统一 snake_case | `imgPath` → `img_path` | 🟡 字段名变化 |
| Task 去除 DimensionName 注入 | 不再自动覆盖维度名 | 🟢 新增字段 |
| 新增原始状态文本字段 | 如 `CircleTaskStatus`、`StatusName` 等 | 🟢 只加不减 |

---

## 详细变更

### 1. 时间字段：全部改为 `string`

#### 为什么

前端 `E:\newCC\life-new2026\nazhi\src` 的所有时间字段都**直接使用服务端返回的原始字符串**：
- `creationTimeStr` — 直接渲染到 `<p class="tima_b">{{item.creationTimeStr}}</p>`
- `circle_date`、`get_date`、`getDateStr` — 直接显示
- `comment_time` — 直接显示
- `exhangeTimeStr` — 直接显示

服务端返回的格式是 `"2026-01-12"`（纯日期）或 `"2026-01-12 14:30:00"`（日期时间），**不是 RFC3339 格式**。SDK 强行用 `time.Time` 解码会导致：
1. 反序列化失败（如果服务端返回非 RFC3339 格式）
2. 序列化时输出格式变了（`YYYY-MM-DD` → `RFC3339`）

#### 变更清单

| 类型 | 字段 | v1.x（当前） | v2.0.0（新） |
|------|------|-------------|-------------|
| `CircleRecord` | `CircleDate` | `time.Time` | `string` |
| `CircleRecord` | `CircleBeginDate` | `string` ✅ | `string` |
| `CircleRecord` | `CircleEndDate` | `string` ✅ | `string` |
| `CircleRecord` | `ObtainTime` | `string` ✅ | `string` |
| `CircleRecord` | `CreationTimeStr` | `string`（v1.3 新增）✅ | `string` |
| `Task` | `StartDate` | `DateOnly` | `string` |
| `Task` | `EndDate` | `DateOnly` | `string` |
| `Task` | `AuditStartDate` | `DateOnly` | `string` |
| `Task` | `AuditEndDate` | `DateOnly` | `string` |
| `Task` | `CreationTimeStr` | `DateOnly` | `string` |
| `Task` | `CreationTime` | `[]int` ✅ | `[]int` （保持数组格式） |
| `HonorRecord` | `GetDate` | `time.Time` | `string` |
| `Notification` | `CreateTime` | `time.Time` | `string` |
| `ExamResult` | `CreateTime` | `time.Time` | `string` |
| `BonusInfo` | `CreateTime` | `time.Time` | `string` |
| `ViolationRecord` | `CreateTime` | `time.Time` | `string` |
| `DemocraticActivity` | `CreateTime` | `time.Time` | `string` |

#### `DateOnly` 类型删除

`pkg/types/datetime.go` 中的 `DateOnly` 自定义类型将被删除，不再需要序列化/反序列化钩子。

---

### 2. 字段命名：全部改为与服务端一致

#### 为什么

通过 HAR 抓包数据和前端源码双重验证，发现服务端**不同 API 使用不同的命名风格**，这不是一个统一的系统：

**getStudentCircle（写实记录列表）** — 大部分 snake_case + 少数 camelCase：
```
[snake_case]  operator_name, circle_task_name, host_name, check_result,
               patent_type, patent_num, term_name, activity_name, sports_name,
               team_name, org_name, results_name, obtain_time,
               specialty_technology, play_role, like_specialty1/2/3,
               circle_date, circle_begin_date, circle_end_date, type_name,
               type, if_my_self, circle_task_id, circle_type_id, dimension_id
[camelCase]   creationTimeStr, imgList, imgPreViewList, commentList,
               auditRemark, likeStatus
```

**getCircleStatistics（任务列表）** — 全部 camelCase：
```
id, name, schoolId, circleTypeId, typeName, startDate, startDateStr,
endDate, endDateStr, auditStartDate, auditStartDateStr, score, remark,
creator, creationTime, creationTimeStr, creatorName, roleName,
scopeType, scopeTypeName, termId, hours, pushNum, upPic,
circleTaskStatus, ...
```

**getCircleTypeByTaskId（任务元数据）** — 全部 snake_case：
```
task_name, circle_type_id, hours, type_name, dimension_id,
dimension_name, task_id, remark, type
```

**getTypicalCase（典型案例列表）** — 全部 camelCase：
```
termName, typeName, title, teacherName, partnerName, statusName,
roleName, remark, content, levelName, attachmentId, attachmentName, ...
```

**getHonorByStudentId（荣誉列表）** — 大部分 snake_case：
```
student_name, class_name, type_name, level_name, get_date,
evaluation_agency, cert_img_attachment_id, statusName, ...
```

**getMyInfo（用户信息）** — 混合（不同环境可能是 snake_case 或 camelCase）：
```
name, className, studentNumber, telephone, genderName, birthdayStr,
youthLeagueFlag, nation, nationalStudentNumber, familyAddress, hobbies,
idCard, idType, seat, ...
```

**v2.0.0 的策略**：每个 API 的结构体 JSON tag 与服务端实际返回的字段名完全一致，不做任何规范化转换。

**注意**：POST 请求体（如 `addCircle`、`addHonor`）前端发送的是 camelCase，所以 SDK 发出的请求**体字段名保持当前的 camelCase 不变**。**只改接收响应数据的 GET 方法的结构体 JSON tag**。

#### 变更范围

所有 `pkg/types/` 下的 JSON tag 需要对照服务端实际响应逐一确认。

根据 HAR 抓包数据（`test/integration/har_fixtures/task_flow.json`），服务端 `getStudentCircle` 响应的实际字段名：

**snake_case**：`id`, `name`, `content`, `type_name`, `status`, `circle_date`, `hours`, `operator_name`, `host_name`, `rank`, `level`, `circle_begin_date`, `circle_end_date`, `check_result`, `patent_type`, `patent_num`, `address`, `term_name`, `activity_name`, `sports_name`, `team_name`, `org_name`, `results_name`, `obtain_time`, `specialty_technology`, `play_role`, `like_specialty1/2/3`

**camelCase**：`creationTimeStr`, `imgList`, `imgPreViewList`, `commentList`, `auditRemark`, `likeStatus`, `ifMySelf`

典型变更示例：

```go
// v1.x（全部 camelCase）
type CircleRecord struct {
    TypeName       string    `json:"typeName"`            // 服务端实际返回 type_name
    CircleDate     time.Time `json:"circleDate"`          // 服务端实际返回 circle_date
    HostName       string    `json:"hostName,omitempty"`
    CircleTaskName string    `json:"circleTaskName"`
    OperatorName   string    `json:"operatorName"`
    IsMySelf       bool      `json:"isMySelf"`
    AuditRemark    string    `json:"auditRemark"`         // 服务端实际返回 auditRemark（本身就是 camelCase！）
    LikeStatus     bool      `json:"likeStatus"`          // 服务端实际返回 likeStatus
    CreationTimeStr string   `json:"creationTimeStr"`     // 服务端实际返回 creationTimeStr
    CircleBeginDate string   `json:"circleBeginDate,omitempty"` // 服务端实际返回 circle_begin_date
    CheckResult    string    `json:"checkResult"`
}

// v2.0.0（与服务端完全一致，保留原始命名风格）
type CircleRecord struct {
    TypeName       string     `json:"type_name"`           // snake_case → 匹配服务端
    CircleDate     string     `json:"circle_date"`         // string + snake_case
    HostName       string     `json:"host_name"`           // snake_case
    CircleTaskName string     `json:"circle_task_name"`    // snake_case
    OperatorName   string     `json:"operator_name"`       // snake_case
    IfMySelf       int        `json:"ifMySelf"`            // int + 保持 camelCase（服务端如此！）
    AuditRemark    string     `json:"auditRemark"`          // camelCase（服务端如此！）
    LikeStatus     int        `json:"likeStatus"`           // int + 保持 camelCase（服务端如此！）
    CreationTimeStr string    `json:"creationTimeStr"`      // camelCase（服务端如此！）
    CircleBeginDate string    `json:"circle_begin_date"`    // snake_case
    CheckResult    string     `json:"check_result"`         // snake_case
}
```

---

### 3. 状态值：保留原始类型

#### 为什么

前端需要原始状态值来做判断：

```javascript
// 前端 managementRightBottom.vue：需要原始 status 值判断
item.status == 2      // "被撤回"判断，展示撤回原因
item.ifMySelf == 1    // "是否自己发布"判断，显示编辑/删除按钮
item.likeStatus       // 点赞状态（0/1）
```

v1.x 将这些 int/string 转成了 bool，丢失了区分多种状态的能力。

#### 变更清单

| 类型 | 字段 | v1.x | v2.0.0 | 原因 |
|------|------|------|--------|------|
| `CircleRecord` | `Status` | 无 → `Approved bool` | `Status int` 保留原始值 | 前端需要 `status==2` 判断撤回 |
| `CircleRecord` | `IfMySelf` | `IsMySelf bool` | `IfMySelf int` | 前端检查 `item.ifMySelf==1` |
| `CircleRecord` | `LikeStatus` | `LikeStatus bool` | `LikeStatus int` | 前端检查 `item.likeStatus` 真假 |
| `Task` | `Submitted` | `Submitted bool` | 保留 `Submitted bool` + 新增 `CircleTaskStatus string` | 前端可能需要原始状态文本 |
| `Task` | `NeedPic` | `NeedPic bool` | `UpPic int` + 保留 `NeedPic bool` | 原始 int 值 |
| `HonorRecord` | `Approved` | `Approved bool` | `Status int` + 保留 `Approved bool` | 原始状态码 |

**设计原则**：保留原始字段的原始类型，新增的语义化字段（如 `Submitted bool`）作为辅助保留。

---

### 4. 删除 postProcessUserInfo

#### 为什么

`pkg/client/user.go:88-115` 的 `postProcessUserInfo` 做了两件事：

1. **学校名 SSO 降级**：`SchoolID==0` 时调用 `GetSchoolID` 外部查询补全
   - 问题：制造了意外的网络请求，学校名可能被修改
2. **班级名前缀清理**：`"高一(8)班"` → `"(8)班"`
   - 问题：修改了原始数据，调用方想获取原始 `ClassName` 时拿到的是被处理过的值

#### 变更

```go
// v1.x
func (c *Client) postProcessUserInfo(ctx context.Context, v *types.UserInfo) {
    if v.StudentNumber != "" && (v.SchoolID == 0 || v.SchoolName == "") {
        // 调用外部 API 补全
    }
    if v.ClassName != "" && v.GradeName != "" && strings.HasPrefix(v.ClassName, v.GradeName) {
        // 去掉年级前缀
    }
}

// v2.0.0：删除该函数，不修改原始数据
```

---

### 5. CircleImage JSON tag 统一

`pkg/types/circle.go` 中 `CircleImage` 的 JSON tag 目前混用：

| 字段 | v1.x | v2.0.0 |
|------|------|--------|
| `ID` | `id` | `id` |
| `CircleID` | `circle_id` ✅ | `circle_id` |
| `ClassID` | `class_id` ✅ | `class_id` |
| `TaskID` | `task_id` ✅ | `task_id` |
| `AttachmentID` | `attachment_id` ✅ | `attachment_id` |
| `ImgPath` | `imgPath` ❌ | `img_path` |

---

### 6. 删除 DimensionName 自动注入

`pkg/client/task.go:276` 中 `tasks[i].DimensionName = dim.Name` 用遍历结果覆盖了可能由服务端返回的维度名。

v2.0.0 不再做这个注入。如果 `getCircleStatistics` 响应中包含 `dimension_name`，它会通过原始 JSON 透传；如果不包含，则 SDK 也不应主动添加。

---

## 迁移步骤

### 第 1 步：更新依赖

```bash
go get github.com/Wenaixi/nazhi-cli@v2.0.0
```

### 第 2 步：时间字段处理

```go
// v1.x：time.Time 用法
record.CircleDate.Format("2006-01-02")
record.CircleDate.Year()

// v2.0.0：string 用法
strings.Split(record.CircleDate, "-")[0]  // 取年份
record.CircleDate                          // 直接是 "2026-01-12" 原始格式
```

#### 常见模式迁移

| 场景 | v1.x | v2.0.0 |
|------|------|--------|
| 取年份 | `t.CircleDate.Year()` | `strings.Split(t.CircleDate, "-")[0]` 或直接用字符串 |
| 格式化为 `YYYY-MM-DD` | `t.CircleDate.Format("2006-01-02")` | 已经是 `"2026-01-12"`，无需格式化 |
| 比较日期先后 | `t.CircleDate.Before(other)` | 字符串字典序比较（YYYY-MM-DD 格式天然支持） |
| 传给前端 API | `json.Marshal` → RFC3339 | 直接透传原始字符串 |

### 第 3 步：字段名更新

```go
// v1.x：camelCase JSON 字段名
json.Unmarshal(data, &record)
fmt.Println(record.TypeName)    // Go 字段名不受影响

// v2.0.0：snake_case JSON 字段名
json.Unmarshal(data, &record)
fmt.Println(record.TypeName)    // Go 字段名不变！
```

**重要**：Go 结构体字段名 **不变**，只改 JSON tag。所以 Go 代码中可以继续用 `record.TypeName`、`record.HostName`。**只有通过 `json.Marshal` 序列化输出的 JSON 字段名会变**。

如果你直接用 `json.Marshal` 输出给外部系统（非 nazhi 平台），需要更新消费者的字段名解析：

```go
// v1.x JSON 输出
{"typeName": "社会实践", "hostName": "学校"}

// v2.0.0 JSON 输出
{"type_name": "社会实践", "host_name": "学校"}
```

### 第 4 步：状态字段更新

```go
// v1.x：bool 判断
if record.Approved { ... }

// v2.0.0：int 判断
if record.Status == 1 { ... }  // 1=已通过
if record.Status == 2 { ... }  // 2=被撤回（新增语义！）
```

#### 状态值枚举

| 字段 | 值 | 语义 |
|------|----|------|
| `CircleRecord.Status` | `0` | 待审核 |
| | `1` | 已通过 / 已结束 |
| | `2` | 被撤回 |
| `CircleRecord.IfMySelf` | `0` | 他人发布 |
| | `1` | 自己发布 |
| `CircleRecord.LikeStatus` | `0` | 未点赞 |
| | `1` | 已点赞 |
| `Task.UpPic` | `0` | 不需要图片 |
| | `1` | 需要上传图片 |
| `Task.CircleTaskStatus` | 字符串 | 如 `"未提交"`、`"已提交"`、`"审核中"`、`"已通过"` 等 |

### 第 5 步：移除对 postProcess 的依赖

```go
// v1.x：ClassName 已被 SDK 清理
fmt.Println(record.ClassName)  // "(8)班"

// v2.0.0：ClassName 保持原始值
fmt.Println(record.ClassName)  // "高一(8)班"
// 需要自己处理前缀
className := strings.TrimPrefix(record.ClassName, record.GradeName)
```

#### 学校名补全

v2.0.0 不再自动调用 `GetSchoolID` 补全校名。如果需要完整的学校信息：

```go
// v2.0.0：自己查
if info.SchoolID == 0 || info.SchoolName == "" {
    schoolInfo, err := c.GetSchoolID(ctx, info.StudentNumber)
    if err == nil {
        // 自行处理
    }
}
```

---

## 涉及文件清单

| 文件 | 变更内容 |
|------|---------|
| `pkg/types/datetime.go` | **删除** `DateOnly` 自定义类型 |
| `pkg/types/circle.go` | `CircleDate` string；混用命名：snake 活动字段 + camelCase UI 字段（`imgList`/`ifMySelf`/`likeStatus`/`auditRemark`/`commentList`/`creationTimeStr`/`showName`/`imgPath`/`studentId`）；`IfMySelf int`（非 bool） |
| `pkg/types/task.go` | `StartDate/EndDate` string、删除 `DateOnly`、`NeedPic`→新增 `UpPic int`、`Submitted` 保留+保留 `CircleTaskStatus` |
| `pkg/types/honor.go` | `GetDate` string、`Approved bool`→保留 `Status int` |
| `pkg/types/notification.go` | `CreateTime` string |（v1.3.0 已删除，不再维护）
| `pkg/types/exam.go` | `CreateTime` string |（v1.3.0 已删除，不再维护）
| `pkg/types/bonus.go` | `CreateTime` string |（v1.3.0 已删除，不再维护）
| `pkg/types/violation.go` | `CreateTime` string |（v1.3.0 已删除，不再维护）
| `pkg/types/democratic.go` | `CreateTime` string |（v1.3.0 已删除，不再维护）
| `pkg/types/deref.go` | 可能不再需要（DateOnly 场景） |
| `pkg/client/user.go` | 删除 `postProcessUserInfo` |
| `pkg/client/task.go` | 删除 `DimensionName` 注入、图片要求检测建议放到 CLI 层 |
| `pkg/client/self_eval.go` | `normalizeSelfEvalStatus` 保留（字段名归一化是读取侧的合理抽象） |

---

## 不变的部分

以下内容 **不** 受影响：

| 部分 | 原因 |
|------|------|
| `envelope` 格式 | 输出包装不变 |
| 退出码三分契约 | 不变 |
| 哨兵错误体系 | 不变 |
| `ActivateSession` 流程 | 不变 |
| `Login` 流程 | 不变 |
| `Option` 模式 | 不变 |
| `*JSON` 方法 | 已经返回原始 JSON，但需要确认新的 snake_case 是否影响 |
| CLI 命令 | CLI 命令注册和流程不变，只有输出的字段名变化 |
| SDK 方法签名 | 所有 `(ctx, token, ...)` 方法签名不变，返回类型可能变（如 `time.Time` → `string`） |

---

## 建议适配策略

### 对于 SDK 直接调用方（Go 项目）

1. 全局搜索 `time.Time` / `DateOnly` / `.Format(` / `time.Parse` 相关调用，改为字符串操作
2. 全局搜索 `.Approved` / `.Submitted` / `.NeedPic` / `.IsMySelf` / `.LikeStatus`，改为 `Status`/`CircleTaskStatus`/`UpPic`/`IfMySelf`/`LikeStatus` 的 int 判断
3. 全局搜索 `ClassName` 使用处，确认是否依赖了 SDK 的前缀清理
4. `json.Marshal` 输出变 snake_case —— 如果下游系统解析了 JSON 字段名，需要同步更新

### 对于 CLI 调用方（Shell 脚本）

1. CLI 输出的字段名变了：`"typeName"` → `"type_name"`，需要更新 `jq` 过滤器
2. CLI 输出的时间格式变了：时间现在是原始字符串而非 RFC3339

```bash
# v1.x
nazhi task list | jq '.data[0].typeName'

# v2.0.0
nazhi task list | jq '.data[0].type_name'
```

---

## 时间线建议

| 步骤 | 内容 | 建议时间 |
|------|------|---------|
| 1 | 发布 v2.0.0-beta 版本（snake_case + string 时间 + 原始状态） | 第 1 周 |
| 2 | 已有 SDK 用户验证兼容性，提交反馈 | 第 2 周 |
| 3 | 修复 beta 反馈问题 | 第 3 周 |
| 4 | 发布 v2.0.0 正式版 | 第 4 周 |
