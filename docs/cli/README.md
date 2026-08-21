# CLI 参考

nazhi-cli：统一 JSON envelope 输出，便于脚本。字段级 API 见 [SDK 分册](../sdk/README.md)。

## 全局选项

| 标志 | 说明 |
|------|------|
| `-v, --verbose` | 详细日志 → stderr |
| `--quiet` | 关闭 stderr（含错误 JSON） |
| `-h, --help` | 帮助 |
| `--version` | 版本 |

优先级：**命令行 > 环境变量 > 默认值**。`flagChanged`：显式 `--token ""` 不会被 `NAZHI_TOKEN` 覆盖。

## 环境变量速查

> 已完成一次脱敏云端登录冒烟：设置 `NAZHI_SILICONFLOW_API_KEY` 后，CLI 成功完成 SSO 登录并返回 200 envelope。真实密钥、账号和 token 不写入仓库。

| 变量 | 用途 | 适用 |
|------|------|------|
| `NAZHI_USERNAME` / `NAZHI_PASSWORD` | 登录 | `login` |
| `NAZHI_TOKEN` | 业务 token | session / whoami / task / circle / honor / typical-case / user / self-eval |
| `NAZHI_SSO_BASE` | SSO 根 | login、file download（默认 `https://www.nazhisoft.com`） |
| `NAZHI_BASE_URL` | 业务 API | 业务命令（默认见发布说明） |
| `NAZHI_UPLOAD_URL` | 上传服 | file upload |
| `NAZHI_TIMEOUT` | 超时秒 | 全局（upload/download 默认更长） |
| `NAZHI_SILICONFLOW_API_KEY` | Nazhi-auto 同款硅基流动 Qwen3-Omni 验证码识别（正式配置，必填） | `login` |
| `NAZHI_OCR_API_KEY` / `SILICONFLOW_API_KEY` | 视觉模型密钥兼容别名 | `login` |
| `NAZHI_OCR_BASE_URL` / `NAZHI_OCR_MODEL` | 可选覆盖视觉模型地址 / 模型名 | `login` |

`login` 使用注入的视觉识别器；未配置视觉模型密钥时返回 503 和 `ErrOCRNotConfigured`。`file upload` / `file download` **不读**业务 token。可用 `.env`（已 gitignore）或 CI secrets。

## 命令树

```
nazhi
├── login
├── session activate
├── whoami
├── user info | update
├── task list | submit | edit | submitted|done | teacher | withdrawn | public | dimensions | circle-type
├── circle delete | comment | like | types | tasks | images | dict
├── self-eval submit | status | grad-status | grad-submit
├── honor types | type-options | level-options | list | add | delete | update | levels
├── typical-case submit | list | update | delete | delete-batch
├── file upload | download
├── version
└── completion
```

## 输出约定

```json
{"status":"success","code":200,"message":"","data":{}}
```

| status | 含义 | 退出码 |
|--------|------|--------|
| success | 成功 | 0 |
| partial | 部分成功 | 1 |
| error | 失败 | 1 业务 / 2 5xx 网络 / 3 参数(400) |

stdout = envelope；stderr = 错误 JSON（非 quiet）+ verbose 日志。

> 双层 code 对照（易混淆提醒）：业务层 `UnifiedResponse.code==1` 等价 CLI 信封层 `envelope.code==200`，同名不同层，切勿混用。
>
> | 层次 | 类型 | 成功值 | 失败值 | 位置 |
> |------|------|--------|--------|------|
> | CLI 信封 | envelope.code | 200 | 4xx/5xx | pkg/envelope.Envelope.Code（HTTP 风格） |
> | 业务响应 | UnifiedResponse.code | 1 | 非 1 | pkg/types.UnifiedResponse.Code |
>
> 脚本判成功请以 `jq '.status=="success"'` 或 `jq '.code==200'` 为准，不要用 `jq '.code==1'` 去判断外层信封。

## 命令速查

| 命令 | 关键 flag | 示例 |
|------|-----------|------|
| `login` | `-u` `-p` | `NAZHI_SILICONFLOW_API_KEY=sk-... nazhi login -u 学号 -p '***'` |
| `session activate` | `--token` | `nazhi session activate --token "$T"` |
| `whoami` / `user info` | `--token` | `nazhi whoami --token "$T"` |
| `user update` | `--token --payload` | `nazhi user update --token "$T" --payload '{"telephone":"13800138000"}'` |
| `task list` | `--token` | `nazhi task list --token "$T"` |
| `task submit` | `--token --payload` `[--address] [--level]` | 见下 |
| `task edit` | `--token --payload` | 见下 |
| `task submitted\|done\|teacher\|withdrawn\|public` | `--token` `[--key] [--limit] [--offset] [--count]` | `nazhi task submitted --token "$T" --key ""` |
| `task dimensions` | `--token` | `nazhi task dimensions --token "$T"` |
| `task circle-type` | `--token --task-id` | `nazhi task circle-type --token "$T" --task-id 18154` |
| `circle delete` | `--token --id` | `nazhi circle delete --token "$T" --id 5400001` |
| `circle comment` | `--token --id --content` | `nazhi circle comment --token "$T" --id 5400001 --content '好'` |
| `circle like` | `--token --id` | `nazhi circle like --token "$T" --id 5400001` |
| `circle types` | `--token --dimension-id [--pid]` | `nazhi circle types --token "$T" --dimension-id 14` |
| `circle tasks` | `--token --type-id` | `nazhi circle tasks --token "$T" --type-id 9274` |
| `circle images` | `--token [--page] [--page-size]` | `nazhi circle images --token "$T" --page 1` |
| `circle dict` | `--token --cate-code` | `nazhi circle dict --token "$T" --cate-code 23` |
| `self-eval submit` | `--token` `--comment` 或 stdin / `--payload`（二者不可同时提供） | `echo 评语 \| nazhi self-eval submit --token "$T"` |
| `self-eval status` | `--token` | `nazhi self-eval status --token "$T"` |
| `self-eval grad-status` | `--token` | `nazhi self-eval grad-status --token "$T"` |
| `self-eval grad-submit` | `--token --comment` 或 stdin | `nazhi self-eval grad-submit --token "$T" --comment "毕业感言"` |
| `honor types` | `--token` | `nazhi honor types --token "$T"` |
| `honor type-options` | `--token` | 类型下拉，读取 `dataList` |
| `honor level-options` | `--token` | 通用等级下拉，读取 `returnData` |
| `honor list` | `--token` list 可 `--key` | `nazhi honor list --token "$T" --page 1` |
| `honor add` | `--token --payload` | `@honor.json` |
| `honor delete` | `--token --id` | `nazhi honor delete --token "$T" --id 1` |
| `honor update` | `--token --payload` | `@honor-update.json` |
| `honor levels` | `--token --type-id` | `nazhi honor levels --token "$T" --type-id 1147` |
| `typical-case submit` | `--token --payload` | `@case.json` |
| `typical-case list` | `--token` `[--status] [--page-size]` | 默认 status=3、page-size=10（与前端一致） |
| `typical-case update` | `--token --payload` | 含 `id` 的 JSON |
| `typical-case delete` | `--token --id` | `nazhi typical-case delete --token "$T" --id 1` |
| `typical-case delete-batch` | `--token --payload` | `nazhi typical-case delete-batch --token "$T" --payload '[1,2,3]'` |
| `file upload` | 路径 / `--file`（无 token） | `nazhi file upload -f ./a.jpg` |
| `file download` | `--id --output`（无 token） | `nazhi file download --id 5139876 -o ./a.jpg` |
| `version` / `completion` | | `nazhi version` |

`--payload` 支持对象 JSON、`@file.json`、`-`（stdin）；顶层 JSON 必须是对象，`null`、数组等非对象输入会按参数错误处理（退出码 3）。stdin payload 最多读取 16 MiB，超过限制会按参数错误处理，不会静默截断。`self-eval submit --payload` 对应前端结构化「诉得失」表单；显式传入空值（`--payload=`）也会立即按参数错误处理，不会退回 stdin/纯文本模式；SDK 会将表单 JSON 序列化后再放入 `studentComment`；例如：

```bash
nazhi self-eval submit --token "$T" --payload '{"bxqhzr":"本学期会做人目标","bxqbx":"本学期表现","bxqys":"优势"}'
```

学期自评用 `submit/status`；毕业评价用 `grad-submit/grad-status`，分别透传 SDK `SubmitSelfGradEvaluation` 与 `QuerySelfGradEvaluationJSON`。

---

## 短示例

### login → session → whoami

```bash
export T=$(nazhi login -u 2025001 -p '***' | jq -r .data.token)
nazhi session activate --token "$T"
nazhi whoami --token "$T" | jq .data
```

### task submit / edit

```bash
# address/level 可选覆盖；空则原样，不默认学校名 / 5
nazhi task submit --token "$T" --payload '{"taskId":18154,"content":"今日劳动。","address":"操场","level":"5","playRole":"3"}'
nazhi task edit --token "$T" --payload '{"id":5400001,"taskId":18154,"content":"补充。","address":"操场","level":"5"}'
```

SDK 自动：任务元数据、图片上传。用户按活动类型填 address/level 等；CLI 解析 `--payload` 时，`cmd/nazhi` 私有 JSON helper 可接收真实前端编辑表单的 number/string 字段：`hours` 的 number 可为小数，`level`、`checkResult`、`playRole` 的 number 必须是有限整数，并会把 `1.0`、`1e0` 等合法整数规范为标准十进制代码字符串，随后统一为提交字段所需的字符串；同时把 `circleTaskId` / `pictureList` 兼容为 `taskId` / `imageIDs`。详见 [sdk/task.md](../sdk/task.md)。

### task 列表

```bash
nazhi task submitted --token "$T" --key "" --limit 20
nazhi task public --token "$T" --count   # 仅总数类用法以 --help 为准
```

### circle

```bash
nazhi circle comment --token "$T" --id 5400001 --content "写得扎实"
nazhi circle like --token "$T" --id 5400001
nazhi circle delete --token "$T" --id 5400001
```

### honor / typical-case

```bash
nazhi honor add --token "$T" --payload @honor.json
# honor.json: typeId, level, evaluationAgency, getDate, certImgAttachmentId(number/string，可选)
# typeName/name/score 可省略（SDK 补）

nazhi typical-case submit --token "$T" --payload @case.json
# type/role/level 用代码；typeName 等可省略
# type 2=社会调查报告，level 1=国际

nazhi typical-case list --token "$T" --status 3 --page-size 10
# 前端初始 attachmentId:"" 会按无附件处理；typeName 等展示名由 SDK 自动补全
nazhi typical-case update --token "$T" --payload '{"id":1,"title":"新标题","type":"1","role":"1","level":"5","teacherName":"王","partnerName":"李","remark":"r","content":"c"}'
nazhi typical-case delete --token "$T" --id 1
```

### user update

```bash
nazhi user update --token "$T" --payload '{"telephone":"13800138000","genderName":"男"}'
# 友好键走 UpdateMyInfoStructured；勿写只读全国学号
```

### file

```bash
nazhi file upload -f ./photo.png          # 无 --token
nazhi file download --id 5139876 -o ./out.jpg
```

> 统一说明：图片压缩后 5MB（SDK 放宽），非图片附件 2MB（与前端一致）。

---

## 完整工作流（最短）

```bash
export T=$(nazhi login -u "$NAZHI_USERNAME" -p "$NAZHI_PASSWORD" | jq -r .data.token)
nazhi session activate --token "$T"
nazhi task list --token "$T" | jq '.data[0].id'
nazhi task submit --token "$T" --payload "{\"taskId\":18154,\"content\":\"体会。\",\"address\":\"操场\",\"level\":\"5\"}"
```

---

## jq 提示

```bash
nazhi whoami --token "$T" | jq -r .data.name
nazhi task submitted --token "$T" | jq '.data.records // .data | length'
```

## 退出码

| 码 | 含义 |
|----|------|
| 0 | 成功 |
| 1 | partial / 业务 4xx |
| 2 | 网络 / 5xx |
| 3 | 参数错误（缺 token、坏 JSON 等） |

---

## SDK 自动补全对照

CLI 是 SDK 薄壳：下列行为在命令里**不用**手填，由 SDK 完成。完整表见 [sdk/autofill.md](../sdk/autofill.md)。

| 场景 | CLI 侧 | SDK 自动 |
|------|--------|----------|
| `login` | 学号/密码（`NAZHI_*` 或 `-u/-p`） | 空 schoolId → 按学号查学校；调用视觉识别器处理验证码 |
| `whoami` / `user info` | token | Session 预热；若 school 不全则用**平台返回的学号**补 schoolId/schoolName |
| `task submit` / `edit` | payload 里 taskId+content+活动字段 | 任务元数据 id；hours 半自动；ImagePaths 上传；**不**默认 address/level |
| `honor add` | typeId/level/agency/getDate | typeName 反查；name 回落 typeName；score=0 |
| `typical-case submit` | type/role/level 代码 + 正文 | *Name 按 code；type2=社会调查报告，level1=国际 |
| `user update` | 友好 JSON 键 | 中文→代码；忽略全国学籍号 |
| `file upload` | 本地路径 | 图片压缩后 ≤5MB（SDK 放宽），非图片附件 ≤2MB（与前端一致）；**不携带**任何鉴权头 |
| 多数业务命令 | token | 内部 `ActivateSession` |

**不会**自动：用默认学号登录、把学号写进写实 body、空地址填学校名、空 level 填 `"5"`。

字段级说明：[SDK 文档中心](../sdk/README.md)。  
