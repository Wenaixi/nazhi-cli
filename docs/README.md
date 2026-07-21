# nazhi-cli 文档中心

文档随 `main` 同步。代码注释见 `pkg/client`、`pkg/types`、`pkg/tokenparse`。

## 用户文档

| 文档 | 内容 |
|------|------|
| [项目 README](../README.md) | 安装、快速开始 |
| [CLI 参考](./cli/README.md) | 命令、环境变量、envelope、短示例 |
| [SDK 参考](./sdk/README.md) | 总览 + **按功能分册** |
| [CHANGELOG](../CHANGELOG.md) | 版本变更 |
| [贡献指南](../CONTRIBUTING.md) | PR / CI |
| [安全策略](../SECURITY.md) | 漏洞与 PII |

## SDK 分册

| 域 | 链接 |
|----|------|
| 认证 | [sdk/auth.md](./sdk/auth.md) |
| Session | [sdk/session.md](./sdk/session.md) |
| 用户 | [sdk/user.md](./sdk/user.md) |
| 任务提交 | [sdk/task.md](./sdk/task.md) |
| 写实列表 | [sdk/circle-list.md](./sdk/circle-list.md) |
| 写实互动 | [sdk/circle-action.md](./sdk/circle-action.md) |
| 荣誉 | [sdk/honor.md](./sdk/honor.md) |
| 典型案例 | [sdk/typical-case.md](./sdk/typical-case.md) |
| 自我评价 | [sdk/self-eval.md](./sdk/self-eval.md) |
| 文件 | [sdk/file.md](./sdk/file.md) |
| 原始 JSON | [sdk/raw-json.md](./sdk/raw-json.md) |
| **自动补全总表** | [sdk/autofill.md](./sdk/autofill.md) |

## 按角色

- **CLI 自动化**：README → [cli/README.md](./cli/README.md)  
- **Go 集成**：[sdk/README.md](./sdk/README.md) → 对应分册  
- **贡献者**：[CONTRIBUTING.md](../CONTRIBUTING.md)

## 版本

文档对齐 **v1.3.0** 及当前 main 行为。历史 tag 下 docs 以 Git 为准。

## 规范摘要

- 中文为主；不堆空话  
- 示例用占位学号/token，禁止真实凭据  
- 写入口文档须区分「用户填 / SDK 自动」；细则见 [sdk/autofill.md](./sdk/autofill.md)  
- 章节深度 ≤ 4  
