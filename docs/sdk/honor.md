# 荣誉申报域

荣誉类型、列表与申报。对应 `pkg/client/honor.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `GetHonorTypes` | 荣誉类型说明表 | `honor types` |
| `GetHonorTypeOptions` | 类型下拉（dataList） | — |
| `GetHonorTypeForSelect` | 级别下拉（returnData） | — |
| `GetHonorLevel` | 按 typeId 联动级别 | — |
| `GetHonorList` | 已申报分页 | `honor list` |
| `AddHonor` | 申报 | `honor add` |
| `UpdateHonor` | 更新 | — |
| `DeleteHonor` | 删除 | `honor delete` |

## 使用方法

```go
err := c.AddHonor(ctx, token, types.AddHonorPayload{
    TypeID:           1147,
    Level:            5,
    EvaluationAgency: "示例中学",
    GetDate:          "2026-06-30",
    // TypeName/Name/Score 可省略
})
```

---

## GetHonorTypes / GetHonorList

```go
types, err := c.GetHonorTypes(ctx, token)
list, err := c.GetHonorList(ctx, token, 1, 10, "") // key 可空
```

### 列表响应字段（snake 为主）

```json
{
  "records": [
    {
      "id": 1,
      "type_name": "校三好学生",
      "level_name": "校",
      "level": 5,
      "dimension_name": "思想品德",
      "status": 1,
      "statusName": "通过",
      "get_date": "2026-06-30",
      "evaluation_agency": "示例中学",
      "score": 5
    }
  ],
  "page": { "pageNo": 1, "pageSize": 10, "totalNum": 1, "totalPage": 1 }
}
```

`HonorType` 说明表：`dimension_name` / `level_name` / `score`。

---

## AddHonor

### 签名

```go
func (c *Client) AddHonor(ctx context.Context, token string, payload types.AddHonorPayload) error
```

### 用户输入 vs SDK 自动

| 用户 | SDK 自动 |
|------|----------|
| TypeID、Level、EvaluationAgency、GetDate、CertImgAttachmentID（先 UploadFile） | TypeName 空则反查；Name 空则回落 TypeName；Score 默认 0 并序列化 |

前端「荣誉名称」是 typeId 下拉，`form.name` 输入已注释。

### 请求示例

```go
up, _ := c.UploadFile(ctx, "./cert.jpg")
err := c.AddHonor(ctx, token, types.AddHonorPayload{
    TypeID:              1147,
    Level:               5,
    EvaluationAgency:    "示例中学",
    GetDate:             "2026-06-30",
    CertImgAttachmentID: strconv.FormatInt(up.AttachmentID, 10),
})
```

### 响应示例

成功 `nil`。

---

## UpdateHonor / DeleteHonor

```go
err := c.UpdateHonor(ctx, token, map[string]any{
    "id": 1, "typeId": 1147, "level": 5,
    "evaluationAgency": "示例中学", "getDate": "2026-06-30",
    // typeName 可空，SDK 补全
})
err = c.DeleteHonor(ctx, token, 1)
```

## 相关类型

- `AddHonorPayload`、`HonorRecord`、`HonorType`、`HonorSelectOption`、`HonorListResult`  
