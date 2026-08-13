# 自我评价域

学期自评与毕业评价。对应 `pkg/client/self_eval.go`。

## 方法一览

| 方法 | 说明 | CLI |
|------|------|-----|
| `SubmitSelfEvaluation` | 纯文本自评 | `self-eval submit` |
| `SubmitSelfEvaluationStructured` | 结构化（诉得失 form） | `self-eval submit --payload` |
| `QuerySelfEvaluation` | 查询自评+教师评语 | `self-eval status` |
| `QuerySelfGradEvaluation` / `QuerySelfGradEvaluationJSON` | 毕业评价查询 | `self-eval grad-status` |
| `SubmitSelfGradEvaluation` | 毕业评价提交 | `self-eval grad-submit` |

## 使用方法

```go
_ = c.SubmitSelfEvaluation(ctx, token, "本学期我认真完成了各项任务。")
st, err := c.QuerySelfEvaluation(ctx, token)
// 未提交时 st == nil && err == nil
```

---

## 请求路径与响应约定

| 方法 | HTTP 请求 | 请求体 / 返回值 |
|------|-----------|----------------|
| `SubmitSelfEvaluation` | `POST /api/studentMoralEduNew/addSelfEvaluation` | `{"studentComment":"评语"}`；成功返回 `nil` |
| `SubmitSelfEvaluationStructured` | `POST /api/studentMoralEduNew/addSelfEvaluation` | `studentComment` 是 `JSON.stringify(form)` 后的字符串；成功返回 `nil` |
| `QuerySelfEvaluation` | `GET /api/studentMoralEduNew/querySelfEvaluation` | 从 `dataMap` 读取 `student_comment`/`teacher_comment`；未提交返回 `nil, nil` |
| `QuerySelfGradEvaluation` | `GET /api/studentMoralEduNew/querySelfGradEvaluation` | 从 `dataMap`/`returnData` 返回 `*map[string]any`；常见字段为 `student_comment`、`isGrad` |
| `SubmitSelfGradEvaluation` | `POST /api/studentMoralEduNew/addSelfGradEvaluation` | `{"studentComment":"毕业评语"}`；成功返回 `nil` |

纯文本与毕业评价提交均只有一层 `studentComment` 包装；结构化学期自评才会把表单 JSON 再嵌入该字段。CLI `self-eval grad-status` 透传 `QuerySelfGradEvaluationJSON` 原始对象；`self-eval grad-submit --comment` 透传 `SubmitSelfGradEvaluation`。

---

## SubmitSelfEvaluation

### 签名

```go
func (c *Client) SubmitSelfEvaluation(ctx context.Context, token string, comment string) error
```

### 用户输入 vs SDK 自动

| 用户 | SDK |
|------|-----|
| 评语文本 | POST 一层 `{"studentComment": comment}`；先 `ActivateSession` |

### 请求 / 响应

```go
err := c.SubmitSelfEvaluation(ctx, token, "本学期收获……")
// 成功 nil
```

---

## SubmitSelfEvaluationStructured

前端 `selfgaintloss.vue`：form 对象 **`JSON.stringify` 后再**作为 `studentComment` 字符串提交（双层）。

| 用户 | SDK |
|------|-----|
| `map` 各键（bxqhzr…） | `json.Marshal(form)` → 作为 `studentComment` 字段值 POST |

```go
err := c.SubmitSelfEvaluationStructured(ctx, token, map[string]any{
    "bxqhzr": "会做人目标",
    "bxqhqz": "会求知目标",
    "bxqhsh": "会生活目标",
    "bxqhcz": "会创造目标",
    "bxqbx":  "本学期表现",
    "bxqys":  "优势",
    "bxqls":  "劣势",
    "sxqhzr": "下学期会做人",
    "sxqhqz": "下学期会求知",
    "sxqhsh": "下学期会生活",
    "sxqhcz": "下学期会创造",
})
```
---

## QuerySelfEvaluation

```go
func (c *Client) QuerySelfEvaluation(ctx context.Context, token string) (*types.SelfEvalStatus, error)
```

### 响应示例

平台查询主路径为 **snake_case**（SDK 兼容 camel）：

```json
{
  "id": 1,
  "student_comment": "本学期……",
  "teacher_comment": "继续努力"
}
```

未提交：`(nil, nil)`，勿当错误。

---

## 毕业评价

```go
m, err := c.QuerySelfGradEvaluation(ctx, token) // *map[string]any
_ = c.SubmitSelfGradEvaluation(ctx, token, "毕业感言……")
```

## 相关类型

- `types.SelfEvalStatus`（`UnmarshalJSON` 主读 student_comment/teacher_comment）  
- 总表：[autofill.md](./autofill.md)  
