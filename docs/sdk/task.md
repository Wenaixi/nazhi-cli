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

### 用户输入 vs SDK 自动

| 用户 | SDK 自动 |
|------|----------|
| TaskID、Content、活动相关字段（name/address/level/playRole/hostName/…）、Hours（任务预设≤0 时必填）、ImagePaths/ImageIDs | getCircleTypeByTaskId → circleTaskId/circleTypeId/dimensionId；Hours 在 meta>0 时可空用预设；本地图上传为 pictureList |

**禁止发明默认**：空 `Address`/`OrgName`/`Level` **原样提交**，不填学校名、不默认 `"5"`。  
`CircleDate`/`TermName` 前端无 v-model，兼容字段，勿当必填。

写实等级（字典 cateCode=23，常见）：`1` 国家 … `5` 校 `6` 年段（以服务端字典为准）。

### 请求示例

```go
// 提交
res, err := c.SubmitTask(ctx, token, types.TaskSubmitInput{
    TaskID:     18154,
    Content:    "今日完成卫生值日。",
    Address:    "教学楼",
    Level:      "5",
    PlayRole:   "3",
    ImagePaths: []string{"./photo.jpg"},
})

// 编辑
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
- 任务备注要求图片但未传图 → `ErrInvalidPayload`  
- 业务拒绝 → `ErrBusinessRejected`（`TaskResult` 可能仍带 code/msg）  

---

## GetCircleTypeByTaskID / GetDimensions

```go
meta, err := c.GetCircleTypeByTaskID(ctx, token, 18154)
// meta: circle_type_id, dimension_id, hours, task_id, remark, ...
dims, err := c.GetDimensions(ctx, token)
```

## 相关类型

- `TaskSubmitInput` / `TaskEditInput` / `TaskAddCirclePayload` / `Task` / `TaskResult` / `TaskCircleTypeInfo`  
