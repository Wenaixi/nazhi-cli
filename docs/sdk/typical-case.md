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

### 用户输入 vs SDK 自动（`fillTypicalCaseDisplayNames`）

| 字段 | 用户 | SDK |
|------|------|-----|
| title / type / role / level / teacherName / partnerName / remark / content | 必填侧（对齐前端） | — |
| `attachmentId` / `attachmentName` | 可选；证书图需先 `UploadFile` 再填 id | SDK **不**从本地路径代传（与荣誉一致）；前端初始 `attachmentId:""` 会按无附件处理并省略；通过 `json.Unmarshal` 复用已有 payload 时，未出现在 JSON 中的字段保留原值 |
| typeName / roleName / levelName | 可空 | 空则按 code 映射（下表）；**已填不覆盖** |
| Update 的 map | type/role/level 可为 **string 或 number** | 均能补 *Name |

| code | typeName | roleName | levelName |
|------|----------|----------|-----------|
| 1 | 研究性学习报告 | 负责人 | **国际**（不是「国家」） |
| 2 | **社会调查报告**（不是「社会实践」） | 参与者 | 省 |
| 3 | 艺术创作作品 | — | 市 |
| 4 | 其他 | — | 区县 |
| 5 | — | — | 学校 |

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
    // TypeName/RoleName/LevelName 可省略
})
```

### 响应示例

成功 `nil`。

---

## GetTypicalCaseList

```go
func (c *Client) GetTypicalCaseList(ctx context.Context, token string, pageNo, pageSize int, status ...int) (*types.TypicalCaseListResult, error)
```

`status` 变参默认 **3=全部**（0 未审 / 1 通过 / 2 驳回 / 3 全部）；与前端列表默认一致。CLI `typical-case list` 默认每页 **10** 条，与前端 `pageSize=10` 一致。SDK 方法的 `pageSize` 仍由调用方显式传入。
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
- 总表：[autofill.md](./autofill.md)  
