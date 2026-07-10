# Migration Guide: v0.6.0 → v1.0.0

## 重大变更 (Breaking)

v1.0.0 是破坏性更新。所有 SDK 消费者必须做以下调整:

### 1. 字段重命名 (所有 response)

| 旧字段 | 新字段 | 说明 |
|--------|--------|------|
| `task.circleTaskStatus` (string) | `task.submitted` (bool) | 字符串转 bool |
| `task.upPic` (int 0/1) | `task.needPic` (bool) | int 转 bool |
| `task.startDateStr` (string) | `task.startDate` (time.Time) | string 转 time.Time |
| `task.endDateStr` (string) | `task.endDate` (time.Time) | string 转 time.Time |
| `circle.status` (int 0/1) | `circle.approved` (bool) | int 转 bool |
| `honor.status` (int 0/1) | `honor.approved` (bool) | int 转 bool |
| `honor.statusName` (camelCase) | `honor.approvedName` (camelCase) | 重命名 |
| `self-eval.student_comment` (snake) | `selfEval.studentComment` (camel) | 统一 camelCase |

### 2. 字段删除 (UserInfo 影响最大)

v1.0.0 首发时 UserInfo 从 51 字段精简到 10 字段:

- 删除: initials/pinyin/studyNumber/nationalStudentNumber/seat/seatSort/gender/genderName/nation/idType/idCard/birthday/birthdayStr/telephone/email/qq/wechat/address/nationality/politicalStatus/creationTime/modificationTime 等 41 字段
- 首发保留: id/name/studentNumber/studentId/schoolId/schoolName/gradeId/gradeName/classId/className

后续 v1.0.x 补回了业务侧高频字段，最终保留以下字段：

id / name / studentNumber / studentId / studyNumber / nationalStudentNumber / schoolId / schoolName / gradeId / gradeName / classId / className / seat

### 3. JSON 命名统一 camelCase

所有字段现在都是 camelCase (替代原 snake_case):

- `circle_task_id` → `circleTaskId` (但 CircleRecord 已删除此字段)
- `student_name` → `studentName` (但 HonorRecord/SelfEvalStatus 已删除)
- `get_date` → `getDate`
- `evaluation_agency` → `evaluationAgency`
- `ifshow` → `ifShow` (但已删除)
- `attachment_id` → `attachmentId` (CircleImage 唯一字段)

### 4. CLI 变更

- 删除 `nazhi school` 命令 (学校信息从 whoami 获取)
- 新增 `nazhi task done` 别名 (等价于 `nazhi task submitted`)
- 退出码: 0=成功, 1=业务错误/partial, 2=网络/服务端错误, 3=参数错误

### 5. envelope 格式

所有 CLI 输出现在统一包装:

```json
{
  "status": "success|partial|error",
  "code": 200,
  "message": "",
  "data": { ... }
}
```

`status` 枚举:

- `success` — 业务成功
- `partial` — 部分成功 (如分页合并时一页失败但其他成功)
- `error` — 业务或网络失败

`code` 字段: 业务 code (1=成功) 或 HTTP 状态码 (200/401/500 等)。

`data` 字段: 任意业务载荷 (object / array / scalar)。

## 升级步骤

1. 更新 nazhi-cli 到 v1.0.0: `go get github.com/Wenaixi/nazhi-cli@v1.0.0`
2. 更新代码使用新字段名 (上表)
3. 处理 envelope 格式 (如果脚本解析 stdout)
4. 测试所有调用点

## 升级示例

### SDK 字段迁移

```go
// v0.6.0
tasks, _ := c.FetchTasks(ctx, token)
for _, t := range tasks {
    if t.CircleTaskStatus == "上传期 已提交" {
        // ...
    }
    if t.UpPic == 1 {
        fmt.Println(t.StartDateStr)
    }
}

// v1.0.0
tasks, _ := c.FetchTasks(ctx, token)
for _, t := range tasks {
    if t.Submitted {  // bool 简化
        // ...
    }
    if t.NeedPic {
        fmt.Println(t.StartDate.Format("2006-01-02"))
    }
}
```

### Circle 状态判断

```go
// v0.6.0
for _, r := range records {
    if r.Status == 1 {
        fmt.Println("已通过", r.CircleDate)
    }
}

// v1.0.0
for _, r := range records {
    if r.Approved {
        fmt.Println("已通过", r.CircleDate.Format("2006-01-02"))
    }
}
```

### CLI envelope 解析

```bash
# v0.6.0 — JSON 直接是业务数据
$ nazhi whoami
{
  "id": 123,
  "name": "张三",
  ...
}

# v1.0.0 — envelope 包装
$ nazhi whoami
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "id": 123,
    "name": "张三",
    ...
  }
}
```

如果脚本用 jq 处理:

```bash
# v0.6.0
nazhi whoami | jq '.name'

# v1.0.0
nazhi whoami | jq '.data.name'
```

### 错误处理

```bash
# v0.6.0 — 错误 JSON 到 stderr, 退出码 1
$ nazhi task list
# stderr: {"error": true, "message": "..."}

# v1.0.0 — 错误也走 envelope 到 stderr, 但退出码 1
$ nazhi task list
# stderr: {"status": "error", "code": 500, "message": "...", "data": null}
# exit 1
```

退出码更细:

| 退出码 | 含义 |
|--------|------|
| 0 | 成功 |
| 1 | 业务错误 (code != 1) 或 partial 状态 |
| 2 | 网络/服务端错误 (HTTP 4xx/5xx) |
| 3 | 参数错误 (CLI flag 解析失败) |
