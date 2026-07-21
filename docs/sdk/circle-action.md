# 写实互动与元数据查询

评论、点赞、删除与辅助查询。对应 `pkg/client/circle.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `DeleteCircle` | 删除写实 | `nazhi circle delete` |
| `AddCircleComment` | 评论 | `nazhi circle comment` |
| `SetCircleLike` | 点赞切换 | `nazhi circle like` |
| `GetCircleTypes` | 维度下类别 | — |
| `GetCircleTasks` | 类别下任务 | — |
| `GetCircleImages` | 写实图片分页 | — |
| `GetDictList` | 字典（如等级 cateCode=23） | — |

## 使用方法

```go
_ = c.AddCircleComment(ctx, token, 5400001, "写得很好")
_ = c.SetCircleLike(ctx, token, 5400001)
_ = c.DeleteCircle(ctx, token, 5400001)
```

---

## DeleteCircle

```go
func (c *Client) DeleteCircle(ctx context.Context, token string, circleID int64) error
// GET .../deleteCircle?id=
```

成功 `nil`。

---

## AddCircleComment

### 签名

```go
func (c *Client) AddCircleComment(ctx context.Context, token string, circleID int64, content string) error
```

### 用户输入 vs SDK 自动

| 用户 | SDK |
|------|-----|
| circleId、content | 组装 `{"circleId", "content"}` POST |

### 请求示例

```go
err := c.AddCircleComment(ctx, token, 5400001, "加油")
```

### 响应示例

成功 `nil`。

---

## SetCircleLike

```go
func (c *Client) SetCircleLike(ctx context.Context, token string, circleID int64) error
// GET .../setCircleLikeById?circleId=
```

服务端切换赞/取消赞。

---

## GetCircleTypes / GetCircleTasks / GetCircleImages / GetDictList

```go
types, err := c.GetCircleTypes(ctx, token, dimensionID, "") // pid 可空，已 QueryEscape
tasks, err := c.GetCircleTasks(ctx, token, typeID)
imgs, err := c.GetCircleImages(ctx, token, 1, 20)
dict, err := c.GetDictList(ctx, token, 23) // 写实等级等
```

返回类型为 `[]map[string]any`（平台原始键，未强类型化）。

### 响应示例（字典项示意）

```json
[
  {"name": "国家", "value": "1"},
  {"name": "校", "value": "5"}
]
```

## 相关类型

列表结构化见 [circle-list.md](./circle-list.md)。  
