# CLI 参考

nazhi-cli 提供 10 个用户可见命令 + 全局选项 + 环境变量 fallback。所有命令输出 JSON 到 stdout、错误 JSON 到 stderr（除非 `--quiet` 静默），便于管道处理。

## 全局选项

| 标志 | 说明 |
|---|---|
| `-v, --verbose` | 详细日志输出到 stderr（含 HTTP 请求头 / 响应码 / OCR 流程） |
| `--quiet` | 静默模式，关闭所有 stderr 输出（包括错误 JSON）—— 适合 CI 流水线只看 exit code |
| `-h, --help` | 显示命令帮助 |
| `--version` | 显示版本号（同 `nazhi version`） |

**优先级**：命令行 > 环境变量 > SDK 默认值。`flagChanged()` 守卫区分「没传 flag」和「传了空值」——`--token ""` 不会被 `NAZHI_TOKEN` 环境变量覆盖。

## 命令树

```
nazhi
├── login                          SSO 登录（全自动 OCR）
├── school                          查询学校 ID（无需登录）
├── session
│   └── activate                    激活业务 Session（HAR 4 步）
├── whoami                          获取当前用户信息
├── task
│   ├── list                        列出全维度任务（8 路并发）
│   └── submit                      提交任务（@payload.json 文件读取）
├── self-eval
│   ├── submit                      提交自我评价（支持 stdin）
│   └── status                      查询评价 + 教师评语
├── file
│   └── upload                      上传图片（不接受 --token）
├── version                         显示版本信息
└── completion                      生成 shell 自动补全
```

## 输出约定

| 场景 | 输出 | 退出码 |
|---|---|---|
| 成功 | JSON 对象 / 数组到 stdout | `0` |
| 业务/网络错误 | `{"error": true, "message": "..."}` 到 stderr | `1` |
| 内部 panic | stderr 打印 `debug.Stack()` + 同样 JSON envelope | `1` |
| `--quiet` 下 | stderr 完全静默 | `0` 或 `1` 由 exit code 体现 |

**JSON 缩进**：所有输出 `json.Indent("", "  ")`，**两空格缩进**，方便人眼查看。
**退出码契约**：`printError` 不直接调 `os.Exit`（否则绕过 `defer closeAllClients()` 泄漏 ONNX 临时目录）；
而是标记 `pendingExitCode=1`，由 `main` 统一退出，保证 LIFO 资源清理。

---

## nazhi version

显示版本号（含 Git commit hash，如有）。

```bash
$ nazhi version
0.4.1

$ nazhi --version
nazhi version 0.4.1
```

## nazhi completion

生成指定 shell 的自动补全脚本。

```bash
nazhi completion bash
nazhi completion zsh
nazhi completion fish
nazhi completion powershell
```

支持的 shell：`bash` / `zsh` / `fish` / `powershell`。

**使用示例**：

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

完成 SSO 登录，自动处理 OCR 验证码识别。**包含 5 步网络调用**（详见 [login-flow.md](../login-flow.md)）。

```bash
nazhi login -u 学号 -p 密码
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `-u, --username` | ✅ | `NAZHI_USERNAME` | 学号 |
| `-p, --password` | ✅ | `NAZHI_PASSWORD` | 密码 |
| `--sso-base` | — | `NAZHI_SSO_BASE` | SSO 根地址，默认 `https://www.nazhisoft.com` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `15` |

**输出**：

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9.eyJzdWIiOi...",
  "expires_at": "2026-07-15T08:00:00+08:00"
}
```

> **Token 有效期**：14 天（JWT `exp` 字段），存到环境变量复用直到过期。
>
> **历史字段**：v0.3.3 之前曾输出 `refresh_after`（推荐刷新时间）和 `user_info`（用户基本信息），因全仓 0 引用 + 0 填充已删除。需要用户基本信息请改用 `nazhi whoami`。

**典型错误分支**：

| 错误 | 原因 |
|---|---|
| `errors.ocr_not_configured: OCR 识别器未配置` | 当前构建未启用 `-tags ddddocr`，且没用 `WithCustomOCR` 注入（CLI 路径下用预编译 release 即可） |
| `login rejected: code=-1 学号或密码错误` | 凭据错 |
| `login rejected: 验证码校验失败` | 99 张图都识别不出来（极少见，可能是服务端 captcha 服务挂） |
| `timeout: 请求 https://... 失败` | 网络慢，调大 `NAZHI_TIMEOUT=30` |

---

## nazhi school

根据学号查询学校 ID（**无需登录**）。

```bash
nazhi school -u 学号
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `-u, --username` | ✅ | `NAZHI_USERNAME` | 学号 |
| `--sso-base` | — | `NAZHI_SSO_BASE` | SSO 根地址，默认 `https://www.nazhisoft.com` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `15` |

**输出**：

```json
{
  "school_id": "173",
  "school_name": "福清一中"
}
```

`school_id` 是数字字符串——内部有 `strconv.ParseInt` 校验，非数字会返回 `ErrInvalidPayload`，避免脏数据传给登录流程。

---

## nazhi session activate

激活业务 Session。**Login 后必须调一次**，否则后续业务接口（`whoami` / `task list` / `task submit` / `self-eval`）会返回空数据。

```bash
nazhi session activate --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址，默认 `http://139.159.205.146:8280` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 `15` |

**HAR 对齐 4 步**（内部流程，HAR 抓包验证必须按顺序）：

1. `GET /`（建立后端 Session）
2. `GET /api/studentInfo/getMenu`（Referer: `/homepage?token=xxx`）
3. `GET /api/studentInfo/getMenu`（Referer: `/home`）
4. `GET /api/studentInfo/getMyInfo`（返回完整 UserInfo）

**输出**：完整 40+ 字段 `UserInfo` 对象，参考 [types.go](https://github.com/Wenaixi/nazhi-cli/blob/main/pkg/types/types.go) `UserInfo` 注释。

**典型错误分支**：

| 错误 | 原因 |
|---|---|
| `session activation backoff: in cooldown window` | 上次激活失败后 5 秒内再调（thundering herd 抑制） |
| `business request rejected by server` | 业务拒绝——通常是 token 过期或无效，重新 `nazhi login` |
| `timeout` | 网络慢，调 `NAZHI_TIMEOUT` |

**`--quiet` 下的 backoff 冷却**：session 激活失败 5 秒内重复调用时，stderr 仍输出 `{"status": "cooldown", "message": "请等待 X 秒"}` 让脚本感知（这不污染错误流，`--quiet` 才完全静默——backoff 提示走 `printError` 路径，所以也受 `--quiet` 守卫）。

---

## nazhi whoami

获取当前登录用户完整资料。

```bash
nazhi whoami --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

**输出**：40+ 字段 `UserInfo` JSON 对象（同 `session activate`，包括姓名 / 学号 / 学校 / 年级 / 班级 / 座号 / 联系方式）。

**空数据场景**：业务成功但确实无用户数据时，stdout 输出 `{"status": "empty", "reason": "returnData 和 dataMap 都为空"}` 而不是裸 `null`（v0.3.5+）。

---

## nazhi task list

列出全维度的任务列表（思想品德 / 学业水平 / 身心健康 / 艺术素养 / 社会实践 / 劳动教育 / 军训等）。

```bash
nazhi task list --token "eyJhbGciOiJIUzUxMiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

**输出**：`Task[]` 数组。

```json
[
  {
    "id": 16512,
    "name": "校园劳动",
    "circleTypeId": 9275,
    "typeName": "劳动",
    "dimensionId": 14,
    "dimensionName": "劳动教育",
    "hours": 2.0,
    "circleTaskStatus": "已提交",
    "upPic": 1,
    "startDateStr": "2026-01-12",
    "endDateStr": "2026-02-10",
    "termId": 4
  }
]
```

**部分失败语义**：8 路并发拉各维度，单维度失败不影响其他维度。控制台输出两类 envelope：

```json
// 全部成功
[ {...}, {...} ]

// 部分成功（部分维度业务拒绝）
{
  "status": "partial",
  "tasks": [ {...}, {...} ],
  "error": "..."
}

// 全部失败
{ "error": true, "message": "..." }
```

**`cancelledCount`**：上下文取消触发的失败会带 `cancelled_count` 字段提示是「可重试」的中断而非真正的业务错误。

---

## nazhi task submit

提交一次任务。29 字段 addCircle 请求体**透传不裁剪**，可直接喂从浏览器抓的 body。

```bash
# 方式 1：--payload 字符串
nazhi task submit --token "xxx" --payload '{"circleTaskId":1001,"name":"班会","hours":1}'

# 方式 2：--payload @file.json 从文件读取
nazhi task submit --token "xxx" --payload @task.json

# 方式 3：从 stdin 读取
echo '{"circleTaskId":1001,"name":"班会","hours":1}' | nazhi task submit --token "xxx" --payload -
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--payload` | ✅ | — | 任务 JSON 字符串、`@file.json` 路径，或 `-` 从 stdin 读取 |
| `--base-url` | — | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒） |

**任务类型字段差异**（HAR 验证）：

| 字段 | 劳动 | 军训 | 班会 | 通用 |
|---|---|---|---|---|
| `name` | 任务原名 | `""` | `"班会"` | 任务原名 |
| `level` | `"5"` | `""` | `""` | `""` |
| `checkResult` | `""` | `"1"` | `""` | `""` |
| `address` | 学校名 | 学校名 | 班级名 | `""` |
| `orgName` | 学校名 | 学校名 | `""` | `""` |
| `playRole` | `""` | `""` | `"3"` | `"3"` |
| `hours` | `2.0` | `32.0` | `1.0` | `0.5` |

完整 29 字段定义见 `pkg/types/types.go` `TaskSubmitPayload` 注释。

**典型错误**：`invalid task payload: circleTaskId 和 circleTypeId 不能为空` —— 必填字段缺失，stderr envelope 退出码 1。

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

**stdin 提示**：当 stdin 是 TTY（交互终端）时，stderr 会打印 `请输入评价文本（Ctrl+D 结束）: `——这是 `printPrompt` 直写 stderr，不受 `--verbose` 守卫，受 `--quiet` 和 `isTerminalStdin()` 守卫。

CI 用 stdin：

```bash
printf '很好的学期' | nazhi self-eval submit --token "$TOKEN" --comment -
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

**输出**：

```json
{
  "student_comment": "很好的学期，掌握了...",
  "teacher_comment": "本学期表现优秀",
  "student_name": "张三",
  "student_number": "2025001",
  "class_name": "高三(1)班",
  "grade_name": "高三",
  "school_id": 173,
  "is_grad": "0",
  "id": 12345
}
```

`fallback` 链：`returnData` → `dataMap` → `dataList[0]`，服务端字段命名风格变更时仍能拿到数据。

---

## nazhi file upload

上传图片到文件服务器。**独立公共服务，不需要业务 token**。

```bash
nazhi file upload -f ./photo.jpg
```

| 标志 | 必填 | 环境变量 | 说明 |
|---|---|---|---|
| `-f, --file` | ✅ | — | 本地图片路径 |
| `--upload-url` | — | `NAZHI_UPLOAD_URL` | 上传服务器，默认 `http://doc.nazhisoft.com` |
| `--timeout` | — | `NAZHI_TIMEOUT` | HTTP 超时（秒），**默认 `30`**（其他命令默认 15） |

**不接受 `--token`**：文件服务器独立，发送 token 反而被风控。命令帮助文字明确写「本命令不接受 --token」，
SDK 内部使用独立 `newCleanClient`（无 cookie jar + 禁用重定向）杜绝泄露。

**支持格式**：JPEG / PNG / GIF（自动取首帧）/ WEBP。BMP 需先转换（stdlib 无 BMP 解码）。

**自动预处理**：任意格式 → JPG + 透明合成 → 质量/缩放级联 → ≤ 5MB（不修改原文件，全部在内存完成）。

预处理流程（F8.1 优化）：

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

**输出**：

```json
{
  "id": 12345,
  "path": "./photo.jpg"
}
```

`id` 可用于 `task submit --payload '{..., "pictureList": [12345]}'`。

---

## 退出码

| 退出码 | 含义 | stderr 内容 |
|---|---|---|
| `0` | 成功 | 空 |
| `1` | 通用错误（业务 / 网络 / 用户错） | JSON `{"error": true, "message": "..."}` |
| `2` | 不应出现（panic 已被 recover 转 1） | stderr 含 `debug.Stack()` |

**为什么不用 2 区分 panic**：与 Go runtime 默认不一致（runtime panic 默认 exit 2）。
v0.4.0 把 panic 也归到 1，让 CI 脚本区分「用户错」(1) 与「崩溃」(2) 时不再被误导。

**`--quiet` 与退出码**：`--quiet` 抑制 stderr JSON 输出，**但退出码不变**——CI 流水线只看 exit code 仍能判断成败。

## 完整工作流示例

**CI/CD 流水线**：

```bash
#!/bin/bash
set -e

# 必填凭据（CI 用 secret 注入）
: "${NAZHI_USERNAME:?必须设置}"
: "${NAZHI_PASSWORD:?必须设置}"

# 慢网络下增加超时
export NAZHI_TIMEOUT=60

# 1. 登录拿 token（jq 提取 token 字段）
TOKEN=$(nazhi login | jq -r .token)
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

**本地调试 / verbose**：

```bash
nazhi -v login -u 学号 -p 密码  # 看完整 HTTP 请求头 / OCR 流程
```

**静默管道**：

```bash
# 仅看退出码，不打印 stderr
nazhi --quiet login -u "$U" -p "$P" > /dev/null && echo "登录成功"
```

**返回 JSON 解析**：

```bash
# 谁拿了什么班级
nazhi whoami | jq -r '"班级：\(.className)\n姓名：\(.name)"'

# 提取所有未完成任务
nazhi task list | jq -r '.[] | select(.circleTaskStatus == "未提交") | .name'
```
