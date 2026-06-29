# nazhi-cli 文档中心

欢迎查阅 nazhi-cli 项目文档。

本文档随 `main` 分支同步更新，与 [CHANGELOG](../CHANGELOG.md) 一一对应。上一版文档请查看 [v0.3.5 tag](https://github.com/Wenaixi/nazhi-cli/tree/v0.3.5/docs)。

## 用户文档

| 文档 | 内容 |
|------|------|
| [README.md](../README.md) | 项目主页、快速开始、安装 |
| [CLI 参考](./cli/README.md) | 所有命令详细参数、示例、返回值 |
| [SDK 参考](./sdk/README.md) | Go SDK API 文档（含 `pkg/tokenparse`） |
| [环境变量](./env-vars.md) | NAZHI_* 环境变量完整说明（含 `NAZHI_UPLOAD_URL`） |
| [HAR 集成测试](./har-testing.md) | HAR 驱动测试架构（含 PII 守卫 SHA-256 重写） |

## 架构文档

| 文档 | 内容 |
|------|------|
| [架构总览](./architecture.md) | 双层架构、目录结构、关键决策 |
| [登录流程](./login-flow.md) | SSO 登录、OCR 验证码、Session 激活 |
| [跨平台 OCR](./cross-platform-ocr.md) | 5 平台 onnxruntime 嵌入策略 + Windows DLL 修复 |
| [HAR 反向工程文档](../Nazhi-auto/docs/reverse/) | 55+ API 端点完整清单（来源：v1 抓包） |

## 开发文档

| 文档 | 内容 |
|------|------|
| [开发指南](../CLAUDE.md) | 项目记忆（AI 协作专用，git 忽略） |
| [贡献指南](../CONTRIBUTING.md) | PR 流程、代码规范、push 前 CI 6 步铁律 |
| [CHANGELOG](../CHANGELOG.md) | 版本变更日志 |
| [安全策略](../SECURITY.md) | 漏洞上报方式 + PII 守卫承诺 |

## 外部参考

- [Go 官方文档](https://go.dev/doc/)
- [cobra 用户指南](https://github.com/spf13/cobra)
- [ddddocr Python 版](https://github.com/sml2h3/ddddocr)
- [Microsoft onnxruntime](https://github.com/microsoft/onnxruntime/releases)
