# nazhi-cli 文档中心

欢迎查阅 nazhi-cli 项目文档。

## 📚 用户文档

| 文档 | 内容 |
|------|------|
| [README.md](../README.md) | 项目主页、快速开始、安装 |
| [CLI 参考](./cli/README.md) | 所有命令详细参数、示例、返回值 |
| [SDK 参考](./sdk/README.md) | Go SDK API 文档 |
| [环境变量](./env-vars.md) | NAZHI_* 环境变量完整说明 |
| [HAR 集成测试](./har-testing.md) | HAR 驱动测试架构 |

## 🏗️ 架构文档

| 文档 | 内容 |
|------|------|
| [架构总览](./architecture.md) | 双层架构、目录结构、关键决策 |
| [登录流程](./login-flow.md) | SSO 登录、OCR 验证码、4 步 Session 激活 |
| [跨平台 OCR](./cross-platform-ocr.md) | 5 平台 onnxruntime 嵌入策略 |
| [HAR 反向工程文档](../Nazhi-auto/docs/reverse/) | 55+ API 端点完整清单（来源：v1 抓包） |

## 🔧 开发文档

| 文档 | 内容 |
|------|------|
| [开发指南](../CLAUDE.md) | 项目记忆（AI 协作专用，git 忽略） |
| [贡献指南](../CONTRIBUTING.md) | PR 流程、代码规范、提交规范 |
| [CHANGELOG](../CHANGELOG.md) | 版本变更日志 |
| [安全策略](../SECURITY.md) | 漏洞上报方式 |

## 🔗 外部参考

- [Go 官方文档](https://go.dev/doc/)
- [cobra 用户指南](https://github.com/spf13/cobra)
- [ddddocr Python 版](https://github.com/sml2h3/ddddocr)
- [Microsoft onnxruntime](https://github.com/microsoft/onnxruntime/releases)
