# CLI 命令清单 (v1.0.0)

本文档是 nazhi-cli 命令速查表, 详尽 flag/示例见 [README.md](README.md)。

## 命令树 (v1.0.0)

```
nazhi
├── login                          SSO 登录 (全自动 OCR)
├── session
│   └── activate                    激活业务 Session (HAR 4 步)
├── whoami                          获取当前用户信息 (含 schoolId)
├── task
│   ├── list                        列出全维度任务 (8 路并发)
│   ├── submit                      提交任务 (@payload.json 文件读取)
│   ├── submitted                   获取已提交写实记录 (自动翻页)
│   └── done                        同 task submitted (别名, 1.0.0 新增)
├── self-eval
│   ├── submit                      提交自我评价 (支持 stdin)
│   └── status                      查询评价 + 教师评语
├── honor
│   ├── types                       获取荣誉类型列表
│   ├── list                        获取已申报荣誉记录 (分页)
│   └── add                         申报荣誉 (@payload.json 文件读取)
├── file
│   └── upload                      上传图片 (不接受 --token)
├── version                         显示版本信息
└── completion                      生成 shell 自动补全
```

> **v1.0.0 变更**:
> - 删除 `nazhi school` — 学校信息现从 `nazhi whoami` 返回的 `schoolId` 字段获取
> - 新增 `nazhi task done` 别名 — 与 `task submitted` 完全等价

## envelope 输出格式 (v1.0.0)

所有 CLI 输出统一包装在 envelope 内, 便于脚本解析:

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": { ... 业务载荷 ... }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| status | string | `success` / `partial` / `error` |
| code | int | HTTP 风格状态码 (200/4xx/5xx) |
| message | string | 错误或提示消息, 成功时为空 |
| data | any | 业务载荷, 可为 object / array / scalar |

### status 枚举

| 值 | 场景 |
|----|------|
| `success` | 业务成功 |
| `partial` | 部分成功 (如分页合并时一页失败但其他成功) |
| `error` | 业务或网络失败 |

### 退出码契约 (v1.0.0 三分)

| 退出码 | 含义 |
|--------|------|
| 0 | 成功 (status=success) |
| 1 | 业务错误 (code != 1) 或 partial 状态 |
| 2 | 网络/服务端错误 (HTTP 4xx/5xx) |
| 3 | 参数错误 (CLI flag 解析失败) |

### envelope 输出示例

**成功**:

```bash
$ nazhi whoami
{
  "status": "success",
  "code": 1,
  "message": "",
  "data": {
    "id": 12345,
    "name": "张三",
    "studentNumber": "G350181200912110035",
    "studentId": 67890,
    "schoolId": 11000001,
    "schoolName": "纳智高中",
    "gradeId": 12,
    "gradeName": "高一",
    "classId": 88,
    "className": "八班"
  }
}
```

**业务错误**:

```bash
$ nazhi task list
{
  "status": "error",
  "code": 500,
  "message": "业务错误 (code=500): 未找到数据",
  "data": null
}
# exit 1
```

**网络错误**:

```bash
$ nazhi task list
{
  "status": "error",
  "code": 503,
  "message": "服务端不可用",
  "data": null
}
# exit 2
```

**参数错误**:

```bash
$ nazhi login --unknown-flag
{
  "status": "error",
  "code": 3,
  "message": "unknown flag: --unknown-flag",
  "data": null
}
# exit 3
```

## jq 解析示例

```bash
# 提取数据
nazhi whoami | jq '.data.name'
# → "张三"

# 退出码判断
if [ $(nazhi task list | jq -r '.status') = "success" ]; then
  echo "OK"
fi

# 错误消息捕获
nazhi task list 2>/dev/null | jq -r '.message // "no error"'
```

## 命令速查表

| 命令 | 用途 | 关键 flag | 返回 data 类型 |
|------|------|-----------|----------------|
| `nazhi login` | SSO 登录 | `--token-out` | `{token, expiresAt, fallbackUsed}` |
| `nazhi session activate` | 激活 session | -- | `{status: "activated"}` |
| `nazhi whoami` | 当前用户信息 | -- | `UserInfo` (10 字段) |
| `nazhi task list` | 列出任务 | `--dimension` | `[]Task` (11 字段) |
| `nazhi task submit` | 提交任务 | `--payload @file.json` 或 `-` | `TaskResult` |
| `nazhi task submitted` | 已提交写实 | `--page` `--size` | `[]CircleRecord` (9 字段) |
| `nazhi task done` | 同 submitted | -- | `[]CircleRecord` (别名) |
| `nazhi self-eval submit` | 提交自评 | `--payload -` (stdin) | `SelfEvalStatus` (3 字段) |
| `nazhi self-eval status` | 查询评价 | -- | `SelfEvalStatus` |
| `nazhi honor types` | 荣誉类型 | -- | `[]HonorType` (5 字段) |
| `nazhi honor list` | 已申报荣誉 | `--page` `--size` | `[]HonorRecord` (9 字段) |
| `nazhi honor add` | 申报荣誉 | `--payload @file.json` 或 `-` | `{id, typeName, ...}` |
| `nazhi file upload` | 上传图片 | `--file PATH` | `{id, name, size, url}` |
| `nazhi version` | 版本号 | -- | `{version, commit}` |
| `nazhi completion` | shell 补全 | `bash/zsh/fish/powershell` | shell 脚本字符串 |

## 全局选项

| 标志 | 说明 |
|---|---|
| `-v, --verbose` | 详细日志到 stderr (含 HTTP 请求头 / 响应码 / OCR 流程) |
| `--quiet` | 静默模式, 关闭所有 stderr 输出 — 适合 CI 流水线只看 exit code |
| `-h, --help` | 命令帮助 |
| `--version` | 版本号 (同 `nazhi version`) |

**优先级**: 命令行 > 环境变量 > SDK 默认值。

**v1.0.0 移除**:
- `nazhi school` 命令 (学校信息从 whoami 获取)
