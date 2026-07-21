# 写实列表域

四类写实列表与总数探测。对应 `pkg/client/submitted.go`。

## 方法一览

| 方法 | type | CLI |
|------|------|-----|
| `GetSubmittedCircles` / `PeekSubmittedTotal` | 3 我发布 | `task submitted` / `done` |
| `GetTeacherCircles` / `PeekTeacherTotal` | 2 教师 | `task teacher` |
| `GetWithdrawnCircles` / `PeekWithdrawnTotal` | 4 撤回 | `task withdrawn` |
| `GetPublicCircles` / `PeekPublicTotal` | 1 公示 | `task public` |

签名共性：

```go
func (c *Client) GetSubmittedCircles(ctx context.Context, token string, key string) ([]types.CircleRecord, error)
func (c *Client) PeekSubmittedTotal(ctx context.Context, token string, key string) (int, error)
```

其余三类将 `Submitted` 换成 `Teacher` / `Withdrawn` / `Public`。`key` 为空表示不筛选。

## 使用方法

```go
list, err := c.GetSubmittedCircles(ctx, token, "")
n, err := c.PeekSubmittedTotal(ctx, token, "劳动")
```

结构化方法自动翻页合并；CLI 大列表多用 `*JSON` / `*LimitJSON`（见 [raw-json.md](./raw-json.md)）。

### SDK 自动

| 行为 | 说明 |
|------|------|
| 翻页 | `Get*Circles` 循环 pageNo 合并全量 `[]CircleRecord` |
| Session | 内部按需 `ActivateSession` |
| `key` | 原样透传；空串 = 不筛选 |
| Peek | 只取 total，不拉全量 |

详见 [autofill.md](./autofill.md)。

---

## GetSubmittedCircles（示例，其它三类同形）

### 请求示例

```go
records, err := c.GetSubmittedCircles(ctx, token, "")
// 关键字
records, err = c.GetTeacherCircles(ctx, token, "德育")
```

### 响应示例（单条，**混用 camel/snake**）

```json
{
  "id": 5400001,
  "content": "劳动体会",
  "host_name": "",
  "type_name": "劳动实践",
  "play_role": 3,
  "imgList": [],
  "imgPreViewList": [],
  "commentList": [],
  "likeStatus": false,
  "ifMySelf": 1,
  "auditRemark": "",
  "creationTimeStr": "2026-07-12 10:00:00",
  "showName": "张三",
  "studentId": 10001
}
```

`play_role` 在 Go 中为 `PlayRoleCode`（兼容 number/string，统一成 `"1"`/`"2"`/`"3"`）。

### 错误 / 注意

- 部分页失败时结构化路径尽量聚合错误信息；CLI JSON 路径可能 `partial` envelope  
- `Peek*` 只取总数，不拉全量列表  

## 相关类型

- `types.CircleRecord`、`types.PageBean`、`types.PlayRoleCode`  
