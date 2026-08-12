# 任务 / 写实提交域

任务列表与写实提交、编辑。对应 `pkg/client/task.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `FetchTasks` | 全维度任务列表（并发） | `nazhi task list` |
| `SubmitTask` | 新增写实 | `nazhi task submit` |
| `EditCircle` | 修改写实 | `nazhi task edit` |
| `GetCircleTypeByTaskID` | 提交元数据 | — |
| `GetDimensions` | 维度列表 | — |

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

### 错误 / 注意

- 缺 content / taskId → `ErrInvalidPayload`  
- 任务 hours≤0 且用户未填 Hours → `ErrInvalidPayload`  
- 任务备注含「照片/图片/pdf」且无图 → `ErrInvalidPayload`  
- 业务拒绝 → `ErrBusinessRejected`（`TaskResult` 可能仍带 code/msg）  
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
