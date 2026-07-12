# CLI 参考

nazhi-cli 提供用户可见命令 + 全局选项 + 环境变量 fallback。所有命令统一 envelope 输出到 stdout、错误 JSON 到 stderr（除非 `--quiet` 静默），便于脚本解析。

## 全局选项

| 标志 | 说明 |
|---|---|
| `-v, --verbose` | 详细日志输出到 stderr（含 HTTP 请求头 / 响应码 / OCR 流程） |
| `--quiet` | 静默模式，关闭所有 stderr 输出（包括错误 JSON）—— 适合 CI 流水线只看 exit code |
| `-h, --help` | 显示命令帮助 |
| `--version` | 显示版本号（同 `nazhi version`） |

优先级：命令行 > 环境变量 > SDK 默认值。`flagChanged()` 守卫区分「没传 flag」和「传了空值」——`--token ""` 不会被 `NAZHI_TOKEN` 环境变量覆盖。

## 命令树

```
nazhi
├── login                          SSO 登录（全自动 OCR）
├── session
│   └── activate                    激活业务 Session（HAR 4 步）
├── whoami                          获取当前用户信息（含 schoolId）
├── task
│   ├── list                        列出全维度任务（8 路并发）
│   ├── submit                      提交任务（最小输入模型 + SDK 自动补全）
│   ├── submitted                   获取已提交写实记录（自动翻页）
│   └── done                        同 task submitted（v1.0.0 新增别名；v1.1.2 支持 --limit/--offset/--count）
├── self-eval
│   ├── submit                      提交自我评价（支持 stdin）
│   └── status                      查询评价 + 教师评语
├── honor
│   ├── types                       获取荣誉类型列表
│   ├── list                        获取已申报荣誉记录（分页）
│   ├── add                         申报荣誉（@payload.json 文件读取）
│   └── delete                      删除荣誉记录
├── file
│   ├── upload                      上传图片（不接受 --token）
│   └── download                    下载附件图片（不接受 --token）
├── version                         显示版本信息
└── completion                      生成 shell 自动补全
```

> v1.0.0 移除：`nazhi school` 命令已删除，学校信息现从 `nazhi whoami` 返回的 `schoolId` 字段获取。

## 输出约定

### envelope 格式（v1.0.0）

所有 CLI 输出统一包装：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": { ... 业务载荷 ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | `success` / `partial` / `error` |
| `code` | int | HTTP 风格状态码（200/4xx/5xx） |
| `message` | string | 错误或提示消息，成功时为空 |
| `data` | any | 业务载荷（object / array / scalar） |

### 退出码三分（v1.0.0）

| 退出码 | 含义 |
|--------|------|
| 0 | 成功（status=success） |
| 1 | 业务错误（code != 1）或 partial 状态 |
| 2 | 网络/服务端错误（HTTP 4xx/5xx） |
| 3 | 参数错误（CLI flag 解析失败） |

### envelope 输出示例

成功:

```bash
$ nazhi whoami
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "id": 10086,
    "name": "张三",
    "studentNumber": "G123456789012345678",
    "studentId": 20101,
    "studyNumber": "2508010404",
    "nationalStudentNumber": "G123456789012345678",
    "schoolId": 10001,
    "schoolName": "示例高中",
    "gradeId": 100,
    "gradeName": "高一",
    "classId": 1001,
    "className": "八班",
    "seat": 29
  }
}
```

业务错误:

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

网络错误:

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

参数错误:

```bash
$ nazhi login --unknown-flag
{
  "status": "error",
  "code": 400,
  "message": "unknown flag: --unknown-flag",
  "data": null
}
# exit 3
```

JSON 缩进：所有输出 `json.Indent("", "  ")`，两空格缩进，方便人眼查看。

退出码契约：`printError` 不直接调 `os.Exit`（否则绕过 `defer closeAllClients()` 泄漏 ONNX 临时目录）；而是标记 `pendingExitCode` 由 `main` 统一退出，保证 LIFO 资源清理。

---

## nazhi version

显示版本信息。

```bash
$ nazhi version
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "version": "1.1.4"
  }
}
```

`--version` 全局标志同此命令。

输出为 envelope 格式，`data.version` 字段含当前版本号。

---

## nazhi completion

生成指定 shell 的自动补全脚本。

```bash
nazhi completion bash
nazhi completion zsh
nazhi completion fish
nazhi completion powershell
```

支持的 shell：`bash` / `zsh` / `fish` / `powershell`。

使用示例：

```bash
# Bash
echo 'source <(nazhi completion bash)' >> ~/.bashrc

# Zsh（先加载 compinit）
echo 'source <(nazhi completion zsh)' >> ~/.zshrc

# Fish
nazhi completion fish | source

# PowerShell
nazhi completion powershell | Out-String | Invoke-Expression
```

---

## nazhi login

完成 SSO 登录，自动处理 OCR 验证码识别。包含 5 步网络调用（详见 [login-flow.md](../login-flow.md)）。

```bash
nazhi login -u 学号 -p 密码
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `-u, --username` | ✅ | `NAZHI_USERNAME` | 学号 |
| `-p, --password` | ✅ | `NAZHI_PASSWORD` | 密码 |
| `--sso-base` | — | `NAZHI_SSO_BASE` | SSO 根地址，默认 `https://www.nazhisoft.com` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `15` |

输出：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "token": "eyJhbGciOiJIUzUxMiJ9.eyJzdWIiOiJHMzUwMTgxMjAwOTEyMTEwMDM1I...",
    "expiresAt": "2026-07-24T01:26:37+08:00",
    "fallbackUsed": false
  }
}
```

Token 有效期：14 天（JWT `exp` 字段），存到环境变量复用直到过期。
>
> v1.0.0 精简：`LoginResponse` 仅 3 字段（token / expiresAt / fallbackUsed），用户基本信息请用 `nazhi whoami`。

典型错误分支：

| 错误 | 原因 |
|---|---|
| `OCR 识别器未配置或出错` | 当前构建未启用 `-tags ddddocr`，且没用 `WithCustomOCR` 注入（CLI 路径下用预编译 release 即可） |
| `登录失败: 学号或密码错误` | 凭据错 |
| `登录失败: 验证码校验失败` | 9 张图都识别不出来（极少见，可能是服务端 captcha 服务挂） |
| `登录失败: timeout` | 网络慢，调大 `NAZHI_TIMEOUT=30` |

---

## nazhi session activate

激活业务 Session。Login 后必须调一次，否则后续业务接口（`whoami` / `task list` / `task submit` / `self-eval`）会返回空数据。

```bash
nazhi session activate --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址，默认 `http://139.159.205.146:8280` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `15` |

HAR 对齐 4 步（内部流程，HAR 抓包验证必须按顺序）：

1. `GET /`（建立后端 Session）
2. `GET /api/studentInfo/getMenu`（Referer: `/homepage?token=xxx`）
3. `GET /api/studentInfo/getMenu`（Referer: `/home`）
4. `GET /api/studentInfo/getMyInfo`（返回完整 UserInfo）

输出：envelope 包裹的 `UserInfo`（13 字段），同 `nazhi whoami` 输出。

典型错误分支：

| 错误 | 原因 |
|---|---|
| `session 激活冷却中` | 上次激活失败后 5 秒内再调（thundering herd 抑制），partial 429 |
| `business request rejected by server` | 业务拒绝——通常是 token 过期或无效，重新 `nazhi login` |
| `get_my_info_empty` | 业务成功但无数据，empty 204 |

---

## nazhi whoami

获取当前登录用户精简资料。

```bash
nazhi whoami --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

输出：envelope 包裹的 `UserInfo`（13 字段，v1.0.0 精简版）。

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "id": 10086,
    "name": "张三",
    "studentNumber": "G123456789012345678",
    "studentId": 20101,
    "studyNumber": "2508010404",
    "nationalStudentNumber": "G123456789012345678",
    "schoolId": 10001,
    "schoolName": "示例高中",
    "gradeId": 100,
    "gradeName": "高一",
    "classId": 1001,
    "className": "八班",
    "seat": 29
  }
}
```

空数据场景：业务成功但无数据时，stdout 输出 `{"status":"success","code":204,"message":"get_my_info_empty","data":null}`（`envelope.Empty` 用 `StatusSuccess`，仅 code=204 区分语义，状态仍为 success 走退出码 0）。

---

## nazhi task list

列出全维度的任务列表（思想品德 / 学业水平 / 身心健康 / 艺术素养 / 社会实践 / 劳动教育等）。

```bash
nazhi task list --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

输出：envelope 包裹的 `Task[]` 数组（21 字段，v1.0.0 精简版）。

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": [
    {
      "id": 10001,
      "name": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声",
      "typeName": "参加的艺术活动项目",
      "dimensionName": "艺术素养",
      "hours": 4,
      "score": 1,
      "remark": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声4个小时",
      "submitted": false,
      "needPic": false,
      "startDateStr": "2026-06-30T00:00:00+08:00",
      "endDateStr": "2026-07-30T00:00:00+08:00",
      "auditStartDateStr": "2026-07-31T00:00:00+08:00",
      "auditEndDateStr": "2026-09-30T00:00:00+08:00",
      "creatorName": "林老师",
      "roleName": "班主任",
      "creationTime": [2026, 6, 30, 11, 39, 19],
      "creationTimeStr": "2026-06-30T00:00:00+08:00",
      "termId": 18,
      "pushNum": 1,
      "scopeType": 2,
      "scopeTypeName": "年段任务"
    },
    {
      "id": 10002,
      "name": "诚以立身，信以应考（诚信教育 励志教育）主题班会",
      "typeName": "主题班会",
      "dimensionName": "思想品德",
      "hours": 0.5,
      "score": 1,
      "remark": "心得+照片",
      "submitted": false,
      "needPic": false,
      "startDateStr": "2026-07-10T00:00:00+08:00",
      "endDateStr": "2026-07-18T00:00:00+08:00",
      "auditStartDateStr": "2026-07-19T00:00:00+08:00",
      "auditEndDateStr": "2026-07-22T00:00:00+08:00",
      "creatorName": "王老师",
      "roleName": "班主任",
      "creationTime": [2026, 7, 4, 9, 33, 53],
      "creationTimeStr": "2026-07-04T00:00:00+08:00",
      "termId": 18,
      "pushNum": 0,
      "scopeType": 1,
      "scopeTypeName": "班级任务"
    }
  ]
}
```

部分失败语义：8 路并发拉各维度，单维度失败不影响其他维度。控制台输出 envelope 表达：

```json
{
  "status": "partial",
  "code": 207,
  "message": "fetch_tasks_partial_failure: ...",
  "data": {
    "tasks": [ {...}, {...} ]
  }
}
```

---

## nazhi task submit

提交一次任务。payload 现在是最小必要输入 JSON，SDK 内部自动补齐任务元数据、学校信息、图片上传结果并提交。

```bash
# 方式 1：--payload 字符串
nazhi task submit --token "xxx" --payload '{"taskId":18155,"content":"手握扫帚净校园，春意盎然拂面来。每一次躬身劳动，都是对责任与成长的最好诠释。"}'

# 方式 2：附带图片
nazhi task submit --token "xxx" --payload '{"taskId":18155,"content":"手握扫帚净校园，春意盎然拂面来。每一次躬身劳动，都是对责任与成长的最好诠释。","imagePaths":["./photo.jpg"]}'

# 方式 3：从文件读取，并用 flag 覆盖地点和等级
nazhi task submit --token "xxx" --payload @task.json --address "示例中学" --level 5

# 方式 4：从 stdin 读取
echo '{"taskId":18155,"content":"劳动让我更懂责任。"}' | nazhi task submit --token "xxx" --payload -
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--payload` | ✅ | — | 任务 JSON 字符串、`@file.json` 路径，或 `-` 从 stdin 读取 |
| `--address` | — | — | 地点（覆盖 `payload.address`；未传则默认学校名） |
| `--level` | — | — | 等级（1=国家 2=省 3=地区/市 4=区/县/街道/社区 5=校；未传默认 5） |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

最小输入 JSON 字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `taskId` | int64 | ✅ | 任务 ID |
| `content` | string | ✅ | 心得/感悟 |
| `imagePaths` | string[] | — | 本地图片路径列表，SDK 自动上传后填入 `pictureList` |
| `playRole` | string | — | 承担角色；不传默认空串 |
| `address` | string | — | 地点；不传默认学校名 |
| `level` | string | — | 等级；不传默认 `"5"` |

SDK 内部自动执行：

1. `getCircleTypeByTaskId(taskId)` 获取 `circleTypeId / dimensionId / hours`
2. `whoami / getMyInfo` 获取 `schoolName`
3. 上传 `imagePaths` 并转成 `pictureList`
4. 组装完整 `addCircle` 请求体后提交

成功输出：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "code": 1,
    "msg": "成功"
  }
}
```

失败输出（该任务只能提交一次）：

```json
{
  "status": "error",
  "code": 500,
  "message": "提交任务失败: business request rejected by server\nSubmitTask失败: 业务错误 (code=-1): 发表写实失败，限制本写实活动只能发表1次",
  "data": null
}
```

---

## nazhi task submitted / nazhi task done

获取当前用户已提交的全部写实记录（含正文、图片、审核状态）。自动翻页合并，输出全量数据。`nazhi task done` 是完全等价的别名（v1.0.0 新增）。

```bash
nazhi task submitted --token "eyJhbGciOiJIUzUxMiJ9.xxx"
nazhi task done --token "eyJhbGciOiJIUzUxMiJ9.xxx"      # 同 submitted，别名
nazhi task submitted --limit 5                           # 只取前 5 条
nazhi task submitted --offset 5 --limit 5                # 跳过 5 条取 5 条
nazhi task submitted --count                             # 只看总数
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |
| `--limit` | — | — | 只输出前 N 条（0=全量，v1.1.2 新增） |
| `--offset` | — | — | 跳过前 N 条后再取（配合 --limit，v1.1.2 新增） |
| `--count` | — | — | 只输出记录总数，不拉列表（v1.1.2 新增） |

`--limit`/`--offset` 模式下输出带 `total` + `records` 的结构；`--count` 模式输出 `{"total": N}`；不加参数时全量输出为原始数组（向后兼容）。

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "total": 2,
    "records": [
      {
        "id": 20001,
        "name": "2026年\"青春唱响逐新章，美育涵养润芳华\"班班有歌声",
        "content": "当最后一个音符落下，掌声如潮水般涌来，我才真正理解了\"班班有歌声\"的意义。",
        "typeName": "",
        "approved": false,
        "circleDate": "0001-01-01T00:00:00Z",
        "hours": 4,
        "imgList": [
          {
            "id": 30001,
            "circle_id": 20001,
            "class_id": 1001,
            "task_id": 10001,
            "attachment_id": 6000001,
            "imgPath": ".jpg"
          }
        ],
        "imgPreViewList": [
          "http://www.nazhisoft.com/common/attachment/getImg?id=6000001"
        ],
        "remark": "2026年"青春唱响逐新章，美育涵养润芳华"班班有歌声4个小时"
      }
    ]
  }
}
```

自动翻页：单页就能覆盖绝大多数场景（默认每页 100 条，服务端上限约 500），只有记录超出一页时才自动翻页合并。翻页途中遇到错误时返回已有数据 + 错误信号。

---

## nazhi self-eval submit

提交自我评价文本。

```bash
# 方式 1：--comment 字符串
nazhi self-eval submit --token "xxx" --comment "很好的学期"

# 方式 2：从 stdin 读取（空或 - 触发）
echo "很充实" | nazhi self-eval submit --token "xxx" --comment -
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--comment` | ✅ | — | 评价文本（空值或 `-` 时从 stdin 读取） |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

stdin 提示：当 stdin 是 TTY（交互终端）时，stderr 会打印 `请输入自我评价内容（Ctrl+D 结束）: `——这是 `printPrompt` 直写 stderr，受 `--quiet` 守卫。

输出：

```json
{
  "status": "success",
  "code": 204,
  "message": "自我评价提交成功",
  "data": null
}
```

---

## nazhi self-eval status

查询自我评价状态 + 教师评语。

```bash
nazhi self-eval status --token "xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

输出：envelope 包裹的 `SelfEvalStatus`（3 字段，v1.0.0 精简版）。

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "id": 50001,
    "studentComment": "本学期学习认真，积极参与各项活动。",
    "teacherComment": "表现良好，继续努力。"
  }
}
```

`fallback` 链：`returnData` → `dataMap` → `dataList[0]`，服务端字段命名风格变更时仍能拿到数据。

---

## nazhi file upload

上传图片到文件服务器。独立公共服务，不需要业务 token。

```bash
nazhi file upload -f ./photo.jpg
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `-f, --file` | ✅ | — | 本地图片路径 |
| `--upload-url` | — | `NAZHI_UPLOAD_URL` | 上传服务器，默认 `http://doc.nazhisoft.com` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `30`（其他命令默认 15） |

不接受 `--token`：文件服务器独立，发送 token 反而被风控。命令帮助文字明确写「本命令不接受 --token」参数，SDK 内部使用独立 `newCleanClient`（无 cookie jar + 禁用重定向）杜绝泄露。

支持格式：JPEG / PNG / GIF（自动取首帧）/ WEBP。BMP 需先转换（stdlib 无 BMP 解码）。

自动预处理：任意格式 → JPG + 透明合成 → 质量/缩放级联 → ≤ 5MB（不修改原文件，全部在内存完成）。

输出：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": 5041963
}
```

按附件 ID 下载图片到本地。独立公共服务，不需要业务 token。

```bash
# 下载附件 ID 5000001 到当前目录
nazhi file download --id 5000001 --output ./photo.jpg

# 配合 task submitted 批量下载
nazhi task submitted | jq -r '.data.records[].imgList[].attachment_id' | \
  xargs -I {} nazhi file download --id {} --output ./img_{}.jpg
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--id` | ✅ | — | 附件 ID（int64，来自 `task submitted` 的 `imgList[].attachment_id` 或 `file upload` 的 `id`） |
| `-o, --output` | ✅ | — | 本地保存路径 |
| `--sso-base` | — | `NAZHI_SSO_BASE` | SSO 域名，默认 `https://www.nazhisoft.com` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `30` |

URL 流程：

```
GET https://www.nazhisoft.com/common/attachment/getImg?id=<ID>
  ↓ 302 重定向
GET http://doc.nazhisoft.com/other/M00/.../<image>.jpg
```

不接受 `--token`：下载服务器是公开服务（同上传），不发送任何鉴权头。

安全约束：

- 重定向跟随仅允许 `nazhisoft.com` 同域白名单（防 SSRF 跳转到第三方）
- 重定向次数上限 5（防恶意 Location 循环）
- 服务端返回 0 字节时删除半成品文件（不留垃圾）

预处理流程：

```
1. sniff magic bytes（避免依赖扩展名）
2. 解码 + 透明合成到白底
3. jpeg.Encode(quality=92)
4. 文件 ≤ 5MB？ → 返回
5. 文件 > 2×5MB？ → 跳缩放级联
6. jpeg.Encode(quality=80) → 返回 if ≤ 5MB
7. 缩放级联（resize 不 encode，7×0.7）
8. jpeg.Encode(quality=40) → 返回 if ≤ 5MB
9. 兜底：ErrFileTooLarge
```

输出：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "id": 12345,
    "output": "./photo.jpg"
  }
}
```

`data.id` 可用于后续业务场景引用图片；在任务提交新模型中，直接把本地路径放进 `imagePaths` 即可，SDK 会自动上传并生成 `pictureList`。

输出：

```json
{
  "status": "success",
  "code": 204,
  "message": "下载成功",
  "data": null
}
```

---

## nazhi honor types

获取所有可申报的荣誉类型列表及级别信息。

```bash
nazhi honor types --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

输出：envelope 包裹的 `HonorType[]` 数组（5 字段，v1.0.0 精简版）。

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": [
    {
      "id": 1147,
      "name": "校学生优秀干部",
      "levelName": "校",
      "level": 5,
      "dimensionName": "思想品德"
    }
  ]
}
```

---

## nazhi honor list

获取当前用户已申报的荣誉记录（分页）。

```bash
nazhi honor list --token "eyJhbGciOiJIUzUxMiJ9.xxx"
nazhi honor list --token "xxx" --page 1 --page-size 50
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--page` | — | — | 页码（从 1 开始），默认 `1` |
| `--page-size` | — | — | 每页条数，默认 `20` |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

输出：envelope 包裹，含 `total` / `page` / `pageSize` / `totalPage` / `records`。

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "total": 2,
    "page": 1,
    "pageSize": 20,
    "totalPage": 1,
    "records": [
      {
        "id": 56241,
        "typeName": "阅读之星",
        "levelName": "校",
        "level": 5,
        "dimensionName": "思想品德",
        "approved": true,
        "approvedName": "已通过",
        "getDate": "2026-06-30T00:00:00+08:00",
        "evaluationAgency": "示例中学"
      }
    ]
  }
}
```

---

## nazhi honor add

申报一条荣誉。payload 是 addHonor 请求体 JSON，可用 @file.json 从文件读取，或 - 从 stdin 读取。

```bash
# 方式 1：--payload 字符串
nazhi honor add --token "xxx" --payload '{"name":"校学生优秀干部","typeId":1147,"typeName":"校学生优秀干部","level":5,"evaluationAgency":"示例中学","getDate":"2026-06-30"}'

# 方式 2：--payload @file.json 从文件读取
nazhi honor add --token "xxx" --payload @honor.json

# 方式 3：从 stdin 读取
echo '{"name":"校学生优秀干部","typeId":1147,"level":5}' | nazhi honor add --token "xxx" --payload -
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--payload` | ✅ | — | 荣誉 JSON 字符串，或 `@file.json` 从文件读取，或 `-` 从 stdin 读取 |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

payload 字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `name` | string | ✅ | 荣誉名称（如"校学生优秀干部"） |
| `typeId` | int | ✅ | 荣誉类型 ID（从 honor types 获取） |
| `typeName` | string | ✅ | 荣誉类型名 |
| `level` | int | ✅ | 级别代码（5=校, 4=区县, 3=市, 2=省, 1=国家） |
| `evaluationAgency` | string | ✅ | 颁发机构 |
| `getDate` | string | ✅ | 获得日期（YYYY-MM-DD） |
| `certImgAttachmentId` | string | — | 证书图片附件 ID（先 file upload 上传） |

输出：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "status": "success",
    "msg": "荣誉申报成功"
  }
}
```

输出：

```json
{
  "status": "success",
  "code": 204,
  "message": "荣誉申报成功",
  "data": null
}
```

典型错误：

| 错误 | 原因 |
|---|---|
| `业务错误 (code=-1): 荣誉名称不能为空` | payload 缺字段或参数错 |
| `业务错误 (code=-1): 荣誉级别不能为空` | level 未传或传错 |

---

## nazhi honor delete

按 ID 删除已申报但未审核的荣誉记录。

```bash
nazhi honor delete --token "eyJhbGciOiJIUzUxMiJ9.xxx" --id 123
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--id` | ✅ | — | 荣誉记录 ID |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

输出：

```json
{
  "status": "success",
  "code": 200,
  "message": "",
  "data": {
    "status": "success",
    "msg": "荣誉记录已删除",
    "id": 123
  }
}
```

输出：

```json
{
  "status": "success",
  "code": 204,
  "message": "荣誉记录已删除",
  "data": null
}
```

---

## jq 解析示例

```bash
# 提取数据
nazhi whoami | jq '.data.name'
# → "张三"

# 提取 token
TOKEN=$(nazhi login | jq -r '.data.token')

# 退出码判断
if [ $(nazhi task list | jq -r '.status') = "success" ]; then
  echo "OK"
fi

# 错误消息捕获
nazhi task list 2>/dev/null | jq -r '.message // "no error"'
```

---

## 退出码与 `--quiet`

### 退出码三分的实际表现

| 场景 | exit code | 典型 data |
|------|-----------|-----------|
| `nazhi whoami` 成功 | 0 | UserInfo 对象 |
| `nazhi task list` 部分失败 | 1 | `{"tasks": [...]}` |
| `nazhi whoami` token 无效 | 1 | null |
| `nazhi login` 网络超时 | 2 | null |
| `nazhi login` 缺参数 | 3 | null |

> v1.0.0 之前曾经把 panic 也归到 1。v1.0.0 起 panic = exit 2（与 Go runtime 默认一致）。

### `--quiet` 与退出码

`--quiet` 抑制 stderr JSON 输出，但退出码不变——CI 流水线只看 exit code 仍能判断成败。

---

## 完整工作流示例

CI/CD 流水线：

```bash
#!/bin/bash
set -e

# 必填凭据（CI 用 secret 注入）
: "${NAZHI_USERNAME:?必须设置}"
: "${NAZHI_PASSWORD:?必须设置}"

# 慢网络下增加超时
export NAZHI_TIMEOUT=60

# 1. 登录拿 token（jq 提取 .data.token）
TOKEN=$(nazhi login | jq -r '.data.token')
export NAZHI_TOKEN="$TOKEN"

# 2. 业务操作
nazhi session activate
nazhi whoami
nazhi task list

# 可选：上传图片（独立服务，不需要 NAZHI_TOKEN）
nazhi file upload -f ./photo.jpg

# 可选：提交自我评价
nazhi self-eval submit --comment "自动化测试学期评语"

# Token 14 天有效，下次复用前无需重登录
unset NAZHI_TOKEN  # 清理敏感变量
```

本地调试 / verbose：

```bash
nazhi -v login -u 学号 -p 密码  # 看完整 HTTP 请求头 / OCR 流程
```

静默管道：

```bash
# 仅看退出码，不打印 stderr
nazhi --quiet login -u "$U" -p "$P" > /dev/null && echo "登录成功"
```
