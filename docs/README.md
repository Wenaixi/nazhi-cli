# nazhi-cli 文档中心

本目录是项目文档的总入口。所有文档随 `main` 分支同步，与 [CHANGELOG.md](../CHANGELOG.md) 一一对应。

> 想直接看代码？[pkg/client/](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/client)、[pkg/types/](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/types)、[pkg/tokenparse/](https://github.com/Wenaixi/nazhi-cli/tree/main/pkg/tokenparse) 三个公开包都自带详细中文注释。

## 用户文档（怎么用）

| 文档 | 内容 |
|---|---|
| [README.md](../README.md) | 项目主页：快速开始 + 安装 + 命令概览 + 环境变量速查 |
| [CLI 参考](./cli/README.md) | 10 个命令的 flag / 输出 / 错误码 / 完整工作流 |
| [SDK 参考](./sdk/README.md) | Go SDK API：Client 构造 + 12 方法 + 10 Option + 15 sentinel + 错误处理骨架 |
| [环境变量](./env-vars.md) | NAZHI_* 完整清单 + urlType 分流 + 与 SDK Option 对应表 |

## 架构文档（怎么实现的）

| 文档 | 内容 |
|---|---|
| [架构总览](./architecture.md) | 双层架构、目录结构、关键决策、并发模型、错误链、数据流 |
| [登录流程](./login-flow.md) | SSO 5 步 + 业务 Session 4 步 + 完整时序图 + token 解析规则 |
| [跨平台 OCR](./cross-platform-ocr.md) | 5 平台 onnxruntime 嵌入策略 + Windows DLL 三轮修复演化 + Pool 并发模型 + 可选构建 |

## 开发文档（怎么贡献）

| 文档 | 内容 |
|---|---|
| [HAR 驱动测试](./har-testing.md) | 抓包驱动 fixture + PII SHA-256 守卫反自反性陷阱 + 测试架构 |
| [贡献指南](../CONTRIBUTING.md) | PR 流程、提交规范、**push 前必跑 6 步铁律** |
| [CHANGELOG](../CHANGELOG.md) | 全部版本变更日志 |
| [安全策略](../SECURITY.md) | 漏洞上报 + PII 守卫承诺 + 凭据历史清理说明 |
| [项目记忆](../CLAUDE.md) | AI 协作专用（**git 忽略**），含架构细节与本机凭据 |

## 按角色看

### 我是用户，想用 CLI 自动化

1. 看 [README.md](../README.md) 的「快速开始」
2. 看 [env-vars.md](./env-vars.md) 配置凭据
3. 需要时查 [cli/README.md](./cli/README.md) 找具体命令

### 我是开发者，要用 Go SDK 集成

1. 看 [sdk/README.md](./sdk/README.md) 的「快速开始」
2. 看 [sdk/README.md](./sdk/README.md) 的「错误处理」章节确认怎么用 `errors.Is` 分支
3. 看 [architecture.md](./architecture.md) 了解 SDK 内部状态（如果要扩展）

### 我是贡献者，要改代码

1. 看 [CONTRIBUTING.md](../CONTRIBUTING.md) 的「push 前必跑」6 步铁律
2. 看 [architecture.md](./architecture.md) 了解代码结构
3. 看 [har-testing.md](./har-testing.md) 了解测试体系
4. 提交规范遵循 Conventional Commits

### 我想理解某些模块的工作原理

| 问题 | 看 |
|---|---|
| 登录怎么跑通 OCR 验证码？ | [login-flow.md](./login-flow.md) |
| 为什么 SetBackoff 在锁内读 backoff？ | [architecture.md](./architecture.md) 关键决策 #4 |
| SDK 怎么区分登录错 vs 业务错？ | [sdk/README.md](./sdk/README.md) 错误处理章节 |
| Windows 临时目录怎么自动清理？ | [cross-platform-ocr.md](./cross-platform-ocr.md) Windows OCR 三轮修复 |
| HAR fixture 怎么生成的？ | [har-testing.md](./har-testing.md) 工作原理 |
| CGO-free 怎么构建？ | [cross-platform-ocr.md](./cross-platform-ocr.md) OCR 可选构建 |
| `withDurationGuard` 是什么设计模式？ | [architecture.md](./architecture.md) 关键决策 #1 |

## 文档版本控制

文档随 `main` 分支同步更新。每次发版前会通过 review-tdd 流程增量更新到当前代码。

**历史版本**：[GitHub Releases](https://github.com/Wenaixi/nazhi-cli/releases) 对应 tag 的 docs/ 目录。

| 文档版本 | 对应 code 版本 | 文档冻结 |
|---|---|---|
| v0.4.0+ | v0.4.0 | 随 [Unreleased] 持续更新 |
| v0.3.5 | v0.3.5 | [v0.3.5 tag](https://github.com/Wenaixi/nazhi-cli/tree/v0.3.5/docs) |

## 外部参考

- [Go 官方文档](https://go.dev/doc/)
- [cobra 用户指南](https://github.com/spf13/cobra)
- [Yangbin1322/go-ddddocr](https://github.com/yangbin1322/go-ddddocr) — Go OCR 绑定
- [Microsoft onnxruntime](https://github.com/microsoft/onnxruntime/releases) — 推理引擎
- [Nazhi-auto](https://github.com/Wenaixi/Nazhi-auto) — 上游参考实现（v1 时期）

---

**文档规范**（维护者参考）：

- 中文为主，技术术语保留英文
- 不堆形容词（"强大" / "业界领先" / "无缝集成" → 直接说能做什么）
- 命令示例用真实路径，不用占位符（学号 `2025001` 这种占位是例外）
- 内链必须可达（CI 跑 `markdown-link-check` 兜底）
- 链接到 GitHub 用相对仓库路径（`../CHANGELOG.md` 而非绝对 URL）
- 章节深度 ≤ 4 级（H4 `#` 是上限）
