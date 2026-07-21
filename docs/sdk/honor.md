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

### 用户输入 vs SDK 自动（对照 `AddHonor` / `ensureHonorTypeName`）

| 字段 | 用户 | SDK |
|------|------|-----|
| typeId | 必填（选类型） | — |
| level | 必填 | 前端会按 type 联动默认第一项；SDK **不**自动选 level，须调用方传 |
| evaluationAgency / getDate | 必填 | — |
| certImgAttachmentId | 可选；先 `UploadFile` | 无本地路径自动上传便利（与前端 upload success 写 id 一致） |
| typeName | 可空 | 空且 typeId>0 → `GetHonorTypeOptions`（**dataList** 类型选项）反查 label；找不到 → `ErrInvalidPayload` |
| name | 可空 | 空 → **回落 typeName**（前端新增不传 name，学生名输入已注释） |
| score | 可显式 | **默认 0** 且 `json:"score"` 无 omitempty，零值也会进请求体 |

前端「荣誉名称」UI = typeId 下拉，不是手填 name。

### 请求示例

```go
// 最小：typeName/name/score 全交给 SDK
err := c.AddHonor(ctx, token, types.AddHonorPayload{
    TypeID:           1147,
    Level:            5,
    EvaluationAgency: "示例中学",
    GetDate:          "2026-06-30",
})

// 带证书图
up, _ := c.UploadFile(ctx, "./cert.jpg")
err = c.AddHonor(ctx, token, types.AddHonorPayload{
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
// typeName 可空 → 与 Add 一样反查；name 不会自动回落（前端 update 也不写 name）
err := c.UpdateHonor(ctx, token, map[string]any{
    "id": 1, "typeId": 1147, "level": 5,
    "evaluationAgency": "示例中学", "getDate": "2026-06-30",
})
err = c.DeleteHonor(ctx, token, 1)
```

`GetHonorTypeForSelect` 返回的是**级别**下拉（returnData）；类型下拉请用 `GetHonorTypeOptions`（dataList）——反查 typeName 已走后者。

## 相关类型

- `AddHonorPayload`、`HonorRecord`、`HonorType`、`HonorSelectOption`、`HonorListResult`  
- 总表：[autofill.md](./autofill.md)  
