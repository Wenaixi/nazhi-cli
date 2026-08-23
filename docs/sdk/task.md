# 任务 / 写实提交域

任务列表与写实提交、编辑。对应 `pkg/client/task.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `FetchTasks` | 全维度任务列表（并发） | `nazhi task list` |
| `SubmitTask` | 新增写实 | `nazhi task submit` |
| `EditCircle` | 修改写实 | `nazhi task edit` |
| `GetCircleTypeByTaskID` | 提交元数据 | `task circle-type --task-id` |
| `GetDimensions` | 维度列表 | `task dimensions` |

## 使用方法

```go
tasks, err := c.FetchTasks(ctx, token)
result, err := c.SubmitTask(ctx, token, types.TaskSubmitInput{
    TaskID:  18154,
    Content: "劳动让我体会到责任的重要性。",
    Address: "操场",
    Level:   "5",
    PlayRole: "3",
})
```

---

## FetchTasks

### 签名

```go
func (c *Client) FetchTasks(ctx context.Context, token string) ([]types.Task, error)
```

### 响应示例（单条）

```json
{
  "id": 18154,
  "name": "班级劳动实践",
  "typeName": "劳动实践",
  "dimensionName": "社会实践",
  "hours": 2,
  "score": 5,
  "submitted": false,
  "needPic": true,
  "startDateStr": "2026-01-12",
  "endDateStr": "2026-02-10"
}
```

---

## SubmitTask / EditCircle

### 签名

```go
func (c *Client) SubmitTask(ctx context.Context, token string, input types.TaskSubmitInput) (*types.TaskResult, error)
func (c *Client) EditCircle(ctx context.Context, token string, input types.TaskEditInput) (*types.TaskResult, error)
```

`TaskEditInput` 比提交多必填 `ID`（已有写实记录 id）。

CLI 的 `--payload` 可直接接收真实前端表单 JSON：`hours` 兼容可为小数的 JSON number 和 string；`level`、`checkResult`、`playRole` 的 JSON number 仅接受有限整数，并把 `1.0`、`1e0` 等合法整数规范为标准十进制代码字符串，string 则按调用方原值保留。小数、非有限值和溢出值会被拒绝。`circleTaskId` / `pictureList` 分别兼容为 `taskId` / `imageIDs`，且显式规范字段优先。CLI 解析 payload 时由 `cmd/nazhi` 私有 JSON helper 将可接受的数字值规范为字符串，再按现有 `addCircle` / `editCircle` payload 发送；公开 `TaskSubmitInput` / `TaskEditInput` 仍按普通 Go string 字段赋值。

列表响应中的 `CircleRecord.PlayRole` 是兼容平台 `play_role` number/string 的输出类型；提交输入中的 `TaskSubmitInput.PlayRole` / `TaskEditInput.PlayRole` 仍由用户填写，JSON 解码时只做表示类型归一，不根据任务或学校猜测字段值。

### 用户输入 vs SDK 自动（对照 `buildTaskPayload`）

| 类别 | 字段 | 谁填 |
|------|------|------|
| 必填用户 | `TaskID`、`Content`；Edit 另需 `ID` | 调用方 |
| 活动用户 | name / address / level / playRole / hostName / rank / activityName / sportsName / teamName / orgName / resultsName / obtainTime / specialtyTechnology / likeSpecialty1–3 / checkResult / patentType / patentNum / circleBeginDate / circleEndDate | 调用方按任务类型；**空串原样，不发明值** |
| 半自动 | `Hours`（字符串） | 用户提供有效值时优先；用户空值且 meta.hours>0 → **SDK 用任务预设**；用户空值且 meta≤0 → `ErrInvalidPayload`；非法值 → `ErrInvalidPayload` |
| 纯 SDK | `circleTaskId`、`circleTypeId`、`dimensionId` | `GetCircleTypeByTaskID(TaskID)` |
| 纯 SDK | `pictureList` | `ImageIDs` + 对每个 `ImagePaths` 调 `UploadFile` |
| 兼容非用户 | `CircleDate`、`TermName` | 前端无 v-model；可空保留，勿当必填 |
| 不出现在 body | 学号 / 姓名 / 学校名 | **不会**自动写入；身份靠 token |

**禁止发明默认**（已从 SDK 去掉的旧行为）：

- 空 `Address` / `OrgName` → ~~学校名~~ **不会**  
- 空 `Level` → ~~`"5"`~~ **不会**  

写实等级字典（`GetDictList(cateCode=23)`，常见）：`1` 国家 … `5` 校 `6` 年段（以服务端为准）。

### 请求示例

```go
// 最小：任务预设 hours>0 时可省略 Hours；图片可选
res, err := c.SubmitTask(ctx, token, types.TaskSubmitInput{
    TaskID:     18154,
    Content:    "今日完成卫生值日。",
    Address:    "教学楼", // 活动类型需要时再填；不要指望 SDK 填学校
    Level:      "5",      // 需要时显式传
    PlayRole:   "3",
    ImagePaths: []string{"./photo.jpg"}, // SDK 自动上传并写入 pictureList
})

// 编辑：同样走 buildTaskPayload 自动元数据 + 图片
res, err = c.EditCircle(ctx, token, types.TaskEditInput{
    ID:      5400001,
    TaskID:  18154,
    Content: "补充说明：已拍照。",
    Address: "教学楼",
    Level:   "5",
})
```

### 响应示例

```json
{
  "code": 1,
  "msg": "成功"
}
```

### 校验策略（有意设计，不复制前端 14 分支）

`TaskSubmitInput.Validate()` / `TaskEditInput.Validate()` 仅校验 `TaskID>0` 且 `Content` 非空（编辑另校验 `ID>0`）。
**不**复刻前端 `checkData()` 的 14 分支条件必填；是否满足活动类型必填由调用方保证，与前端保持“前端强校验、SDK 弱校验、服务端终审”一致。

因此调用方提交前需按 `targetName`（活动类型 1-14）自行补齐下表字段，否则服务端将以 `ErrBusinessRejected` 拒绝。前端源码位置：`managementRightBottom.vue` / `managementRightTop.vue` / `yhmanagement/*` 的 `checkData()`。

#### targetName 1-14 必填字段速查（对齐前端 `checkData`）

> 每行均隐含 `Content` 必填；`rank`/`level` 仅在部分类型为“成对校验”：`rank` 非空则 `level` 必填，`level` 非空则 `rank` 必填，见标注。

| targetName | 活动类型（前端注释） | 必填字段 | 备注 |
|------------|---------------------|----------|------|
| 1 | 思想品德-党团/社团/志愿/公益 | `name`、`address`、`hours`、`playRole` |  |
| 2 | 学业水平-学科竞赛成绩 | `activityName`、`hostName`、`obtainTime` | `rank`/`level` 成对校验 |
| 3 | 身心健康-体育比赛项目 | `sportsName`、`hostName`、`obtainTime` | `rank`/`level` 成对校验 |
| 4 | 艺术素养-艺术活动项目 | `name`、`hostName`、`obtainTime` | `rank`/`level` 成对校验 |
| 5 | 艺术素养-学生艺术团队 | `teamName`、`orgName`、`checkResult`、`rank`、`circleBeginDate`、`circleEndDate` | `rank` 单独必填 |
| 6 | 实践创新-勤工俭学/军训/研学/社会调查/生产劳动/参观体验 | `orgName`、`address`、`hours`、`checkResult` |  |
| 7 | 实践创新-科技创新/研究性学习成果 | `resultsName`、`hostName`、`obtainTime` | `rank`/`level` 成对校验 |
| 8 | 实践创新-创造发明成果 | `resultsName`、`patentType`、`obtainTime`、`patentNum` |  |
| 9 | 劳动素养-日常生活/生产/服务性劳动 | `likeSpecialty1` |  |
| 10 | 劳动素养-劳动（组织/地点/时长/级别） | `orgName`、`address`、`hours`、`level` |  |
| 11 | 劳动素养-时间+地点 | `obtainTime`、`address` |  |
| 12 | 劳动素养-劳动特长 | `specialtyTechnology` |  |
| 13 | 劳动素养-劳动成果 | `resultsName`、`address`、`obtainTime` |  |
| 14 | 劳动素养-劳动竞赛情况 | `sportsName`、`hostName`、`obtainTime` | `rank`/`level` 成对校验 |

> 说明：上表字段名与 `TaskAddCirclePayload` JSON 键（camelCase）一致：`circleBeginDate`/`circleEndDate`/`checkResult`/`patentType`/`patentNum`/`likeSpecialty1`/`specialtyTechnology` 等。`hours` 仍走半自动规则：`meta.hours>0` 可空用预设，否则必填。

#### Go 直调 vs CLI 的 number/string 兼容边界

- **Go 直调**：`TaskSubmitInput` / `TaskEditInput` 的 `Hours`、`Level`、`CheckResult`、`PlayRole` 等均为 `string`。若上游为 `number`，调用方须自行 `fmt.Sprintf("%v", v)` 转字符串后再赋值；SDK 不做隐式转换。
- **CLI 边界**：`--payload` 的 JSON 中上述字段的 `number` 兼容仅在 `cmd/nazhi` 私有 helper 生效（有限整数归一为十进制字符串，小数/非有限值拒绝；`hours` 允许小数 number）。此兼容不透传到 SDK 层。

### 错误 / 注意

- 缺 content / taskId → `ErrInvalidPayload`  
- 任务 hours≤0 且用户未填 Hours → `ErrInvalidPayload`  
- 任务备注含「照片/图片/pdf」且无图 → `ErrInvalidPayload`  
- 业务拒绝 → `ErrBusinessRejected`（`TaskResult` 可能仍带 code/msg）  
- 未满足上表 14 分支必填 → 前端会在 `checkData` 拦截；SDK 仅做最小校验，缺字段由服务端以 `ErrBusinessRejected` 返回  
- 总表：[autofill.md](./autofill.md)  

---

## GetCircleTypeByTaskID / GetDimensions

```go
meta, err := c.GetCircleTypeByTaskID(ctx, token, 18154)
// meta: circle_type_id, dimension_id, hours, task_id, remark, ...
dims, err := c.GetDimensions(ctx, token)
```

## 相关类型

- `TaskSubmitInput` / `TaskEditInput` / `TaskAddCirclePayload` / `Task` / `TaskResult` / `TaskCircleTypeInfo`

## 类型注意

- 平台只返回 `upPic`；`FetchTasks` 内 `SetNeedPicFromUpPic()` 推导 `NeedPic`。
