# 原始 JSON 透传（*JSON）

CLI 与上游脚本需要 **平台原始 JSON** 时使用本族方法；`envelope.data` 与 SDK 返回的 `json.RawMessage` 1:1。对应 `pkg/client/raw_json.go` 及部分域内 `*JSON`。

## 方法一览

| 方法 | CLI |
|------|-----|
| `GetSubmittedCirclesJSON` / `GetSubmittedCirclesLimitJSON` | `task submitted` / `done` |
| `GetTeacherCirclesJSON` / `LimitJSON` | `task teacher` |
| `GetWithdrawnCirclesJSON` / `LimitJSON` | `task withdrawn` |
| `GetPublicCirclesJSON` / `LimitJSON` | `task public` |
| `FetchTasksJSON` | —（CLI list 用结构化） |
| `ActivateSessionJSON` | `session activate` |
| `GetMyInfoJSON` | `whoami` / `user info` |
| `QuerySelfEvaluationJSON` | `self-eval status` |
| `GetHonorTypesJSON` / `GetHonorListJSON` | `honor types` / `list` |
| `GetTypicalCaseListJSON` | `typical-case list` |

## 使用方法

```go
raw, err := c.GetMyInfoJSON(ctx, token)
// raw 为平台 body 片段或拼装后的 records/page，勿再假设全 camel

raw, pb, err := c.GetSubmittedCirclesLimitJSON(ctx, token, 0, 20, "")
// offset/limit 分页；pb 为 PageBean
```

写实列表签名均带 `key string`；荣誉列表 `GetHonorListJSON(ctx, token, pageNo, pageSize, key)`。

---

## 与结构化 API 的关系

| | 结构化 | *JSON |
|--|--------|-------|
| 用途 | 类型安全、自动翻页合并 | CLI/脚本原样透传 |
| 解析 | `types.*` tag（须对齐真实键） | 不解析业务字段 |
| 部分失败 | 视方法而定 | 列表类可能 partial |

**注意**：`*JSON` 正确不能替代结构化解析测试；两边都要维护。

---

## 请求 / 响应示例

### GetMyInfoJSON

```go
raw, err := c.GetMyInfoJSON(ctx, token)
os.Stdout.Write(raw)
```

响应为平台 getMyInfo 数据区原始字节（键名与线上一致）。

### GetSubmittedCirclesLimitJSON

```go
raw, page, err := c.GetSubmittedCirclesLimitJSON(ctx, token, 0, 50, "")
// page.TotalNum 等
```

拼装形态示意：

```json
{
  "records": [ /* 原始 dataList 元素 */ ],
  "page": { "pageNo": 1, "pageSize": 50, "totalNum": 3, "totalPage": 1 }
}
```

### GetTypicalCaseListJSON

```go
raw, err := c.GetTypicalCaseListJSON(ctx, token, 1, 10)       // status 默认 3
raw, err = c.GetTypicalCaseListJSON(ctx, token, 1, 10, 1)    // 仅通过
```

## 错误 / 注意

- 分页 Limit 只请求覆盖 offset/limit 的页，避免全量再截断  
- cancel 路径与结构化对称处使用 `ErrRetryable` 等（见各方法注释）  
