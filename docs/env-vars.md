# 环境变量参考

所有 CLI 命令都支持环境变量 fallback，**命令行标志始终优先于环境变量**。SDK Option 模式另有 `WithSSOBase` / `WithBaseURL` / `WithUploadURL` / `WithTimeout` 等，CLI 行为与之对应。

## 完整列表

| 变量 | 作用 | 适用命令 | 默认值 | 兜底无效值 |
|---|---|---|---|---|
| `NAZHI_USERNAME` | 学号 | `login`、`school` | — | 拒绝并 warn |
| `NAZHI_PASSWORD` | 密码 | `login` | — | 拒绝并 warn |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval` | — | 拒绝并 warn |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login`、`school` | `https://www.nazhisoft.com` | 保留默认 |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval` | `http://139.159.205.146:8280` | 保留默认 |
| `NAZHI_UPLOAD_URL` | 文件上传服务器 | `file upload` | `http://doc.nazhisoft.com` | 保留默认 |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 | `15`（`file upload` 是 `30`） | `<=0` 拒绝 |

> **「拒绝并 warn」语义**：CLI 路径下为硬错（`printError` + exit 1），让 CI 立即发现；
> SDK 路径下静默 `c.logger.Warn` 保留当前值。这与 Option 模式的守卫约定对称。

## 优先级规则

```
命令行标志 > 环境变量 > SDK 默认值
```

**关键实现**：用 `cmd.Flags().Changed("name")` 判定用户是否显式传过 flag。
`--token ""` 与 `--token "real"` 都算「显式传过」，不会被 `NAZHI_TOKEN` 环境变量覆盖。
这样 CI 流水线可以放心设 `NAZHI_TOKEN`，同时允许命令行显式覆盖。

## 命令 ↔ 变量映射（urlType 分流）

CLI 在 `cmd/nazhi/opt_builder.go` 的 `buildClientOpts` 按 `urlType` 分流，避免无关变量污染：

| urlType | 命令 | URL 来源 | Token 来源 |
|---|---|---|---|
| `sso` | `login`、`school` | `--sso-base` / `NAZHI_SSO_BASE` | **不读 token**（Login 自带） |
| `base` | `session`、`whoami`、`task`、`self-eval` | `--base-url` / `NAZHI_BASE_URL` | `--token` / `NAZHI_TOKEN`（必填） |
| `upload` | `file upload` | `--upload-url` / `NAZHI_UPLOAD_URL` | **不读 token**（文件服务器独立） |

> **设计要点**：`file upload` 命令根本不注册 `--token` flag——`NAZHI_TOKEN` 即使设了值也会被 `urlType=="upload"` 分支短路（F16 修复 + 回归测试 `TestBuildClientOpts_UploadIgnoresNAZHI_TOKEN`）。
>
> 这避免了「上传临时文件被业务域审计抓取」的安全风险——上传图片不需要业务身份。

## 三种使用方式

### 方式 1：临时环境变量

```bash
export NAZHI_USERNAME=学号
export NAZHI_PASSWORD=密码
nazhi login           # 不需要传 -u/-p
```

适合交互式终端调试。**不要**写到 `~/.bashrc`——shell 历史会留痕，建议用 `~/.bashrc.local`。

### 方式 2：.env 文件（推荐本地开发）

```bash
cp .env.example .env
# 编辑 .env：
#   NAZHI_USERNAME=学号
#   NAZHI_PASSWORD=密码
#   NAZHI_TIMEOUT=30
make test-integration  # 自动读取 .env
```

`.env` 已在 `.gitignore` 中，**绝不**入 git。文件权限建议 `chmod 600 .env`（仅当前用户可读）。

`.env` 通过 `make` / `make test-integration` 自动加载（Makefile 内置 source）。

### 方式 3：CI 注入

```yaml
# GitHub Actions 示例
- name: 集成测试
  env:
    NAZHI_USERNAME: ${{ secrets.NAZHI_USERNAME }}
    NAZHI_PASSWORD: ${{ secrets.NAZHI_PASSWORD }}
    NAZHI_TIMEOUT: 60   # CI 慢网络下增加超时
  run: go test -tags=integration -v ./test/integration/...
```

CI 推荐用 secret 管理（GitHub Actions Secrets / GitLab CI Variables / Vault）。
**绝不**把真实凭据写在 CI 配置文件或 commit message 里。

## URL/Token 进阶用法

### 切换 SSO 服务器（自部署测试）

```bash
NAZHI_SSO_BASE=http://localhost:8080 nazhi login -u test -p test
```

`NAZHI_BASE_URL`、`NAZHI_UPLOAD_URL` 同理——可用于对接内网代理或测试环境。

### 自定义超时（慢网络）

```bash
NAZHI_TIMEOUT=120 nazhi file upload -f big.png
```

`file upload` 默认 30 秒，其他命令默认 15 秒。慢 4G / VPN 下加到 60~120。

### 复用 Token 跨进程

```bash
# 第一次跑：登录 + 拿 token
TOKEN=$(nazhi login | jq -r .token)
echo "$TOKEN" > ~/.nazhi_token  # 仅本地用，chmod 600

# 后续每次跑：直接用缓存的 token（14 天有效）
export NAZHI_TOKEN=$(cat ~/.nazhi_token)
nazhi task list

# Token 过期后重登录
nazhi login > ~/.nazhi_token
```

**为什么不内置 `nazhi token save` 命令**：会增加状态管理负担（写入哪个文件、过期清理、加密存储等），
而环境变量是最简单、最 Unix 的方案。token 文件推荐 chacha20poly1305 加密或只用 1 小时窗口。

## 与 SDK 的对应

CLI 的环境变量 fallback 与 SDK Option 直接一一对应：

| 环境变量 | SDK Option | 备注 |
|---|---|---|
| `NAZHI_SSO_BASE` | `client.WithSSOBase(url)` | 空字符串都拒绝 |
| `NAZHI_BASE_URL` | `client.WithBaseURL(url)` | 同上 |
| `NAZHI_UPLOAD_URL` | `client.WithUploadURL(url)` | 同上 |
| `NAZHI_TIMEOUT` | `client.WithTimeout(d)` | `<=0` 都拒绝 |
| （不暴露） | `client.WithToken(t)` | CLI 用 `NAZHI_TOKEN`，SDK 用 `WithToken` |
| （不暴露） | `client.WithCustomOCR(r)` | SDK 测试 / CGO-free 用 |
| （不暴露） | `client.WithOCRConcurrency(n)` | SDK 高级，CLI 走默认 |
| （不暴露） | `client.WithSessionBackoff(d)` | SDK 高级，调 backoff 窗口 |
| （不暴露） | `client.WithHTTPClient(hc)` | SDK 完全自定义 |
| （不暴露） | `client.WithLogger(l)` | SDK 自定义 slog handler |

作为 SDK 集成方，所有 CLI 环境变量都有对应 Option：

```go
import "github.com/Wenaixi/nazhi-cli/pkg/client"

c, _ := client.New(
    client.WithSSOBase(os.Getenv("NAZHI_SSO_BASE")),
    client.WithBaseURL(os.Getenv("NAZHI_BASE_URL")),
    client.WithTimeout(parseTimeout(os.Getenv("NAZHI_TIMEOUT"))),
    client.WithToken(os.Getenv("NAZHI_TOKEN")),
)
```

## 调试：看哪些变量生效

`--verbose` 模式下 SDK 会输出请求 URL：

```bash
$ NAZHI_BASE_URL=http://localhost:8280 NAZHI_TIMEOUT=30 nazhi -v task list --token xxx
[verbose] → GET http://localhost:8280/api/studentCircleNew/getDimensions
[verbose]   Header: X-Auth-Token: eyJhbGc...
[verbose] ← 200 (340 bytes)
```

如果是 stdout 看到的是默认 URL 而非环境变量的值，说明环境变量未生效——常见原因：
- `export` 写错（`export NAZHI_BASEURL=http://...` 漏下划线）
- `.env` 路径不对（`make test-integration` 在 `nazhi-cli/` 目录跑，`.env` 在同目录）
- 子进程不继承 export（CI runner 配置问题）

## 安全建议

- **绝不**把真实学号/密码写在脚本里
- **用 secret 管理**（GitHub Secrets、Vault、AWS Secrets Manager）
- **.env 文件绝不入 git**（已在 `.gitignore` 第 49 行隔离 + `verify_gitignore/` 兜底测试）
- **临时文件中残留的 token** 已用 `git-filter-repo` 清理历史
- **定期轮换** SSO 密码（历史上 v0.2.0 之前发生过泄露）
- **`NAZHI_TIMEOUT` 调试时不要改太大**：300+ 秒会让 panic recover 后的资源清理卡住

## 历史字段清理

v0.4.0 之前曾存在的环境变量：

| 旧变量 | 状态 | 备注 |
|---|---|---|
| ~~`NAZHI_LOG_LEVEL`~~ | 删除 | 用 `-v` / `-q` 控制 |
| ~~`NAZHI_PROXY`~~ | 未实现 | 如需代理，用 `WithHTTPClient` 注入自定义 Transport |

如有其他环境变量建议，请开 issue。
