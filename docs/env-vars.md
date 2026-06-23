# 环境变量参考

所有 CLI 命令都支持通过环境变量注入凭据和配置。**命令行标志始终优先于环境变量**。

## 完整列表

| 变量 | 作用 | 适用命令 | 默认值 |
|------|------|---------|--------|
| `NAZHI_USERNAME` | 学号 | `login`、`school` | （无） |
| `NAZHI_PASSWORD` | 密码 | `login` | （无） |
| `NAZHI_TOKEN` | X-Auth-Token | `session`、`whoami`、`task`、`self-eval` | （无） |
| `NAZHI_SSO_BASE` | SSO 根地址 | `login`、`school` | `https://www.nazhisoft.com` |
| `NAZHI_BASE_URL` | 业务 API 根地址 | `session`、`whoami`、`task`、`self-eval` | `http://139.159.205.146:8280` |
| `NAZHI_UPLOAD_URL` | 文件上传服务器 | `file upload` | `http://doc.nazhisoft.com` |
| `NAZHI_TIMEOUT` | HTTP 超时（秒） | 所有命令 | `15`（`file upload` 是 `30`） |

## 优先级规则

```
命令行标志 > 环境变量 > SDK 默认值
```

**重要**：使用 `flagChanged()` 判断用户是否显式传了标志，未传才用环境变量。这样用户传 `--timeout 15` 不会被环境变量 `NAZHI_TIMEOUT=30` 覆盖。

## 三种使用方式

### 方式 1：临时环境变量

```bash
export NAZHI_USERNAME=学号
export NAZHI_PASSWORD=密码
nazhi login  # 不需要传 -u/-p
```

### 方式 2：.env 文件（推荐本地开发）

```bash
cp .env.example .env
# 编辑 .env 填入真实凭据
make test-integration  # 自动读取 .env
```

`.env` 已在 `.gitignore` 中，不会被提交。

### 方式 3：CI 注入

```yaml
# .github/workflows/ci.yml
- name: 集成测试
  env:
    NAZHI_USERNAME: ${{ secrets.NAZHI_USERNAME }}
    NAZHI_PASSWORD: ${{ secrets.NAZHI_PASSWORD }}
  run: go test -tags=integration -v ./test/integration/...
```

## 完整示例：CI/CD 流水线

```bash
#!/bin/bash
set -e

# 从 CI secret 读取
export NAZHI_USERNAME="${NAZHI_USERNAME:?必须设置}"
export NAZHI_PASSWORD="${NAZHI_PASSWORD:?必须设置}"
export NAZHI_TIMEOUT=60  # 慢网络下需要更长超时

# 1. 登录拿 token
LOGIN_RESP=$(nazhi login)
TOKEN=$(echo "$LOGIN_RESP" | jq -r .token)
export NAZHI_TOKEN=$TOKEN

# 2. 业务操作
nazhi whoami
nazhi task list

# 3. 提交自我评价
nazhi self-eval submit --comment "自动化测试学期评语"

# 4. 清理（可选，Token 14 天有效）
unset NAZHI_TOKEN
```

## 安全建议

- **不要在脚本中硬编码** 真实学号密码
- **使用 secret 管理**（GitHub Secrets、Vault、AWS Secrets Manager）
- **在 CI 中通过环境变量注入**
- **定期轮换** SSO 密码，因为历史上发生过泄露
- **`.env` 文件绝不入 git**（已在 `.gitignore`）
- **临时文件中残留的 token** 用 `git-filter-repo` 清理历史
