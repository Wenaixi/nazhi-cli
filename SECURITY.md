# 安全策略

## 支持版本

| 版本 | 支持状态 |
|------|---------|
| `0.4.x` | 当前活跃维护（review-tdd 第 18/20/21 轮修复持续合入，未升 minor） |
| `0.3.x` | 仅安全修复 |
| `< 0.3` | 不再支持，请升级 |

> 0.4.x 是 review-tdd 第 15/16/18/20/21 轮全面修复 + 架构深化版：
> OCR Windows 三轮修复（DLL 占用降级 + GOOS 守卫 + 启动清扫 %TEMP%）、token 提取拆 pkg/tokenparse 等多项安全相关改进。
>
> 额外增强（review-tdd 第 18/20/21 轮）：
>
> - sessionManager 4 入口收口 + DCL fast-path
> - 5 个新 HTTP 状态码 sentinel：ErrRateLimited / ErrServiceUnavailable / ErrTimeout / ErrInvalidResponse / ErrRetryable
> - 顶层 panic recover 统一 exit code 1（不再误归 exit 0 或 2）
> - PII 守卫改 SHA-256 哈希反自反性陷阱
>
> 这些改进让 SDK 用户能 errors.Is 精确识别：登录错 / 业务错 / HTTP 层错 / 服务端限流 / ctx cancel 可重试 五种场景。

## 报告安全问题

如果发现安全漏洞，**请不要公开提交 Issue**。请通过以下方式私下报告：

- 联系仓库所有者（GitHub: @Wenaixi）
- 或发送邮件至 GitHub 账号关联邮箱

我们会在合理时间内响应并修复。

## PII（个人可识别信息）守卫承诺

仓库测试与文档 **绝不** 出现真实姓名、学号、密码等 PII 原文。
`test/integration/har_pii_redacted_test.go` 实现了一道 SHA-256 防线：

- 守卫表只存 PII 的 **SHA-256 hex 摘要**（单向不可逆）
- 扫描时对候选字符串计算 SHA-256，与表比对命中即报错
- 历史样本（如真实学生姓名）已用占位符 `TEST2025001` / `TestPass123` 替换

为什么用 hash 而不是明文：早期守卫曾用 PII 字符串本身的子串做自检，
结果守卫文件本身成了新的泄露源。改为不可逆 hex 后，hex 值不可能反推原文。

## 历史凭据清理

早期版本曾有真实学号/密码进入 git 历史（HAR fixture + 集成测试）。
已用 `git-filter-repo` 彻底清除对象库 + force push，并在公开凭据后建议用户
立即修改 SSO 平台密码。**filter-repo 只重写主仓库对象库**——任何 worktree
副本需手动同步，否则会重新泄露。

`CLAUDE.md` 含架构细节和测试凭据占位说明，已写入 `.gitignore` 防止误 track，
并有 `test/integration/verify_gitignore/`（`//go:build verify`）兜底测试。

## 最佳实践

使用 nazhi-cli 时的安全注意事项：

1. **凭证管理** — CLI 接受密码作为命令行参数，注意 shell 历史记录。建议在交互式脚本中使用环境变量。
2. **API Token** — Token 通过 302 重定向安全传输，请勿在不安全网络上明文截获。
3. **OCR 模型** — 所有模型文件嵌入二进制，无运行时下载，避免供应链攻击。
4. **密钥轮换** — 如果账号密码泄露，请立即在目标平台修改密码。
5. **升级建议** — 跨大版本前请阅读 `CHANGELOG.md` 的 BREAKING 项。
