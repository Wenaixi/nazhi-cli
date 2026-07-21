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

| 变量 | 用途 | 适用 |
|------|------|------|
| `NAZHI_USERNAME` / `NAZHI_PASSWORD` | 登录 | `login` |
| `NAZHI_TOKEN` | 业务 token | session / whoami / task / circle / honor / typical-case / user / self-eval |
| `NAZHI_SSO_BASE` | SSO 根 | login、file download（默认 `https://www.nazhisoft.com`） |
| `NAZHI_BASE_URL` | 业务 API | 业务命令（默认见发布说明） |
| `NAZHI_UPLOAD_URL` | 上传服 | file upload |
| `NAZHI_TIMEOUT` | 超时秒 | 全局（upload/download 默认更长） |

`file upload` / `file download` **不读**业务 token。可用 `.env`（已 gitignore）或 CI secrets。

## 命令树

```
nazhi
├── login
├── session activate
├── whoami
├── user info | update
├── task list | submit | edit | submitted|done | teacher | withdrawn | public
├── circle delete | comment | like
├── self-eval submit | status
├── honor types | list | add | delete
├── typical-case submit | list | update | delete
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

## 命令速查

| 命令 | 关键 flag | 示例 |
|------|-----------|------|
| `login` | `-u` `-p` | `nazhi login -u 2025001 -p '***'` |
| `session activate` | `--token` | `nazhi session activate --token "$T"` |
| `whoami` / `user info` | `--token` | `nazhi whoami --token "$T"` |
| `user update` | `--token --payload` | `nazhi user update --token "$T" --payload '{"telephone":"13800138000"}'` |
| `task list` | `--token` | `nazhi task list --token "$T"` |
| `task submit` | `--token --payload` `[--address] [--level]` | 见下 |
| `task edit` | `--token --payload` | 见下 |
| `task submitted\|done\|teacher\|withdrawn\|public` | `--token` `[--key] [--limit] [--offset] [--count]` | `nazhi task submitted --token "$T" --key ""` |
| `circle delete` | `--token --id` | `nazhi circle delete --token "$T" --id 5400001` |
| `circle comment` | `--token --id --content` | `nazhi circle comment --token "$T" --id 5400001 --content '好'` |
| `circle like` | `--token --id` | `nazhi circle like --token "$T" --id 5400001` |
| `self-eval submit` | `--token` `--comment` 或 stdin / `--payload` | `echo 评语 \| nazhi self-eval submit --token "$T"` |
| `self-eval status` | `--token` | `nazhi self-eval status --token "$T"` |
| `honor types\|list` | `--token` list 可 `--key` | `nazhi honor list --token "$T" --page 1` |
| `honor add` | `--token --payload` | `@honor.json` |
| `honor delete` | `--token --id` | `nazhi honor delete --token "$T" --id 1` |
| `typical-case submit` | `--token --payload` | `@case.json` |
| `typical-case list` | `--token` `[--status]` | 默认 status=3 全部 |
| `typical-case update` | `--token --payload` | 含 `id` 的 JSON |
| `typical-case delete` | `--token --id` | `nazhi typical-case delete --token "$T" --id 1` |
| `file upload` | 路径 / `--file`（无 token） | `nazhi file upload ./a.jpg` |
| `file download` | `--id --output`（无 token） | `nazhi file download --id 5139876 -o ./a.jpg` |
| `version` / `completion` | | `nazhi version` |

`--payload` 支持内联 JSON、`@file.json`、`-`（stdin）。

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

SDK 自动：任务元数据、图片上传。用户按活动类型填 address/level 等。详见 [sdk/task.md](../sdk/task.md)。

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
# honor.json: typeId, level, evaluationAgency, getDate, certImgAttachmentId(可选)
# typeName/name/score 可省略（SDK 补）

nazhi typical-case submit --token "$T" --payload @case.json
# type/role/level 用代码；typeName 等可省略
# type 2=社会调查报告，level 1=国际

nazhi typical-case list --token "$T" --status 3
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
nazhi file upload ./photo.png          # 无 --token
nazhi file download --id 5139876 -o ./out.jpg
```

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

更多字段与自动填充行为见 [SDK 文档中心](../sdk/README.md)。  
