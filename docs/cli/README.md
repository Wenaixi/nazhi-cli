# CLI 参考

nazhi-cli 提供 11 个用户可见命令 + 全局选项 + 环境变量配置。

## 全局选项

| 标志 | 说明 |
|------|------|
| `-v, --verbose` | 详细日志输出到 stderr |
| `--quiet` | 静默模式，关闭所有 stderr 输出 |
| `--output json` | 输出格式（当前仅支持 json） |
| `-h, --help` | 显示帮助 |

## 命令树

```
nazhi
├── login                          SSO 登录（全自动 OCR）
├── school                          查询学校 ID（不需登录）
├── session
│   └── activate                    激活业务 Session
├── whoami                          获取当前用户信息
├── task
│   ├── list                        列出全维度任务
│   └── submit                      提交任务
├── self-eval
│   ├── submit                      提交自我评价
│   └── status                      查询评价状态
└── file
    └── upload                      上传图片
├── version                         显示版本信息
└── completion                      生成 shell 自动补全脚本
```

## 通用约定

- **JSON 输出**：成功时输出 JSON 到 stdout
- **错误输出**：失败时输出 `{"error": true, "message": "..."}` 到 stderr，退出码 1
- **`--quiet` 屏蔽**：屏蔽所有 stderr，便于管道处理
- **环境变量**：所有标志都有对应的 `NAZHI_*` 环境变量 fallback

## nazhi version

显示当前版本号。

```bash
nazhi version
nazhi --version    # 等效
```

**输出**：纯文本版本号（如 `0.2.2`）。

## nazhi completion

生成指定 shell 的自动补全脚本。

```bash
nazhi completion bash
nazhi completion zsh
nazhi completion fish
nazhi completion powershell
```

支持的 shell：`bash`、`zsh`、`fish`、`powershell`。

**使用示例**：

```bash
# Bash
source <(nazhi completion bash)

# Zsh（先加载补全系统）
echo "autoload -U compinit; compinit" >> ~/.zshrc
echo "source <(nazhi completion zsh)" >> ~/.zshrc

# fish
nazhi completion fish | source

# PowerShell
nazhi completion powershell | Out-String | Invoke-Expression
```

## nazhi login

完成 SSO 登录，自动处理 OCR 验证码。

```bash
nazhi login -u 学号 -p 密码
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `-u, --username` | ✅ | `NAZHI_USERNAME` | 学号 |
| `-p, --password` | ✅ | `NAZHI_PASSWORD` | 密码 |
| `--sso-base` | ❌ | `NAZHI_SSO_BASE` | SSO 根地址，默认 `https://www.nazhisoft.com` |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时（秒），默认 15 |

**输出**：

```json
{
  "token": "eyJhbGciOiJIUzUxMiJ9...",
  "expires_at": "..."
}
```

> Token 有效期 14 天，存到环境变量复用。
>
> **字段历史注**（v0.3.3+）：旧版本曾输出 `refresh_after`（推荐刷新时间）和
> `user_info`（用户基本信息）两个字段，因全仓 0 引用 / 全 0 填充，于 v0.3.3
> 移除。需要用户基本信息请改用 `nazhi whoami` 命令。

## nazhi school

根据学号查询学校 ID（无需登录）。

```bash
nazhi school -u 学号
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `-u, --username` | ✅ | `NAZHI_USERNAME` | 学号 |
| `--sso-base` | ❌ | `NAZHI_SSO_BASE` | SSO 根地址 |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

**输出**：

```json
{
  "school_id": "173",
  "school_name": "福清一中"
}
```

## nazhi session activate

激活业务 Session。必须先激活，否则后续业务接口返回空数据。

```bash
nazhi session activate --token "eyJhbGciOiJIUzI1NiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | ❌ | `NAZHI_BASE_URL` | 业务 API 根地址，默认 `http://139.159.205.146:8280` |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

**输出**：完整 51 字段用户信息。

## nazhi whoami

获取当前用户完整资料（姓名、学号、学校、年级、班级、座号等）。

```bash
nazhi whoami --token "eyJhbGciOiJIUzI1NiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | ❌ | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

**输出**：51 字段 `UserInfo` 对象。

## nazhi task list

列出全维度任务。

```bash
nazhi task list --token "eyJhbGciOiJIUzI1NiJ9.xxx"
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--base-url` | ❌ | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

**输出**：`Task[]` 数组，每条包含 ID、name、hours、status、dimensionName 等。

## nazhi task submit

提交任务（29 字段 addCircle 请求体）。

```bash
nazhi task submit --token "..." --payload '{"circleTaskId":1001,"name":"班会","hours":1}'
nazhi task submit --token "..." --payload @task.json
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--payload` | ✅ | — | 任务 JSON 字符串或 `@file.json` 路径 |
| `--base-url` | ❌ | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

**任务类型字段差异**（HAR 验证）：

| 字段 | 劳动 | 军训 | 班会 | 通用 |
|------|------|------|------|------|
| `name` | 任务原名 | `""` | `"班会"` | 任务原名 |
| `level` | `"5"` | `""` | `""` | `""` |
| `checkResult` | `""` | `"1"` | `""` | `""` |
| `address` | 学校名 | 学校名 | 班级名 | `""` |
| `playRole` | `""` | `""` | `"3"` | `"3"` |
| `hours` | `2.0` | `32.0` | `1.0` | `0.5` |

## nazhi self-eval submit

提交自我评价文本。

```bash
nazhi self-eval submit --token "..." --comment "很好的学期"
echo "很充实" | nazhi self-eval submit --token "..." --comment -
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `--token` | ✅ | `NAZHI_TOKEN` | X-Auth-Token |
| `--comment` | ✅ | — | 评价文本（`-` 表示从 stdin 读取） |
| `--base-url` | ❌ | `NAZHI_BASE_URL` | 业务 API 根地址 |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

## nazhi self-eval status

查询自我评价 + 教师评语。

```bash
nazhi self-eval status --token "..."
```

**输出**：

```json
{
  "student_comment": "...",
  "teacher_comment": "...",
  "student_name": "...",
  "class_name": "..."
}
```

## nazhi file upload

上传图片到文件服务器。

```bash
nazhi file upload -f ./photo.jpg
```

| 标志 | 必填 | 环境变量 | 说明 |
|------|------|---------|------|
| `-f, --file` | ✅ | — | 本地图片路径 |
| `--upload-url` | ❌ | `NAZHI_UPLOAD_URL` | 上传服务器，默认 `http://doc.nazhisoft.com` |
| `--timeout` | ❌ | `NAZHI_TIMEOUT` | HTTP 超时 |

**不支持 `--token`**：文件服务器独立，发送 token 反而被风控。

**支持格式**：JPEG、PNG、GIF（取首帧）、WEBP。BMP 需先转换。

**自动预处理**：任意格式 → JPG + 透明合成 + 压缩至 ≤ 5MB。

**输出**：

```json
{
  "id": 12345,
  "path": "./photo.jpg"
}
```

## 退出码

| 退出码 | 说明 |
|--------|------|
| `0` | 成功 |
| `1` | 通用错误（详见 stderr JSON） |

## 完整使用流程

```bash
# 1. 登录拿 token
export NAZHI_USERNAME=学号
export NAZHI_PASSWORD=密码
TOKEN=$(nazhi login | jq -r .token)
export NAZHI_TOKEN=$TOKEN

# 2. 激活 Session
nazhi session activate

# 3. 业务操作
nazhi whoami
nazhi task list
nazhi task submit --payload @task.json
nazhi self-eval submit --comment "很好的学期"
nazhi self-eval status

# 4. 上传图片（独立，不需要 token）
nazhi file upload -f ./photo.jpg
```
