# 典型案例域

典型案例 CRUD。对应 `pkg/client/typical_case.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `AddTypicalCase` | 新增 | `typical-case submit` |
| `GetTypicalCaseList` | 分页列表 | — |
| `GetTypicalCaseListJSON` | 原始 JSON | `typical-case list` |
| `UpdateTypicalCase` | 更新（map） | `typical-case update` |
| `DeleteTypicalCase` | 删除 | `typical-case delete` |
| `DeleteBatchTypicalCase` | 批量删除 | — |

## 使用方法

```go
err := c.AddTypicalCase(ctx, token, types.AddTypicalCasePayload{
    Title:       "论 AI 大模型",
    Type:        "1",
    TeacherName: "王老师",
    PartnerName: "李同学",
    Role:        "1",
    Remark:      "简述",
    Content:     "正文……",
    Level:       "5",
})
```

---

## AddTypicalCase

### 用户输入 vs SDK 自动

| 用户 | SDK 自动 |
|------|----------|
| Title、Type、Role、Level、TeacherName、PartnerName、Remark、Content、AttachmentID/Name | 空 typeName/roleName/levelName 按 el-option 补全 |

| code | typeName | roleName | levelName |
|------|----------|----------|-----------|
| 1 | 研究性学习报告 | 负责人 | **国际** |
| 2 | **社会调查报告** | 参与者 | 省 |
| 3 | 艺术创作作品 | — | 市 |
| 4 | 其他 | — | 区县 |
| 5 | — | — | 学校 |

勿与写实列表「国家 / 社会实践」文案混用。

### 请求示例

```go
err := c.AddTypicalCase(ctx, token, types.AddTypicalCasePayload{
    Title: "社会调查：社区志愿服务",
    Type:  "2", // → 社会调查报告
    Role:  "2", // → 参与者
    Level: "1", // → 国际
    TeacherName: "王老师",
    PartnerName: "无",
    Remark: "任务描述",
    Content: "材料内容……",
})
```

### 响应示例

成功 `nil`。

---

## GetTypicalCaseList

```go
func (c *Client) GetTypicalCaseList(ctx context.Context, token string, pageNo, pageSize int, status ...int) (*types.TypicalCaseListResult, error)
```

`status` 默认 **3=全部**（0 未审 / 1 通过 / 2 驳回 / 3 全部）。

### 响应示例

```json
{
  "records": [
    {
      "id": 1,
      "title": "论 AI",
      "type": 1,
      "typeName": "研究性学习报告",
      "role": 1,
      "roleName": "负责人",
      "level": 5,
      "levelName": "学校",
      "status": 0,
      "statusName": "未审核",
      "teacherName": "王老师",
      "content": "……"
    }
  ],
  "page": { "pageNo": 1, "pageSize": 10, "totalNum": 1, "totalPage": 1 }
}
```

列表中 type/role/level 为 **整数**；提交请求体为 **字符串**。

---

## UpdateTypicalCase

```go
err := c.UpdateTypicalCase(ctx, token, map[string]any{
    "id": 1,
    "title": "新标题",
    "type": 2,        // string 或 number 均可补 *Name
    "role": "1",
    "level": 5,
    "teacherName": "王老师",
    "partnerName": "李",
    "remark": "r",
    "content": "c",
})
```

---

## Delete / Batch

```go
_ = c.DeleteTypicalCase(ctx, token, 1)
_ = c.DeleteBatchTypicalCase(ctx, token, []int64{1, 2, 3}) // body 为纯 JSON 数组
```

## 相关类型

- `AddTypicalCasePayload`、`TypicalCaseRecord`、`TypicalCaseListResult`  
- 常量 `TypicalCaseStatusPending/Approved/Rejected/All`  
