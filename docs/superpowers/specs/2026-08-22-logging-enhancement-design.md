# 日志系统增强 — 设计规格

> 来源：头脑风暴 exit_plan_mode 已确认设计；本文件为 writing-plans 所需规格锚点
> 版本：1.3.x 增强 不升大版本 零新依赖 基于 stdlib slog

## 1 目标

将现有 verbose/quiet 二元日志升级为全流程可追踪、结构化、分级、可落盘、可脱敏的日志体系，覆盖 SDK 与 CLI 全链路。

## 2 成功标准

S1 单一 traceId 串起全部 slog 行与 HTTP 行
S2 --log-level debug/info/warn/error 兼容 --verbose；--log-format text/json；--log-file/NAZHI_LOG_FILE 落盘
S3 敏感字段永不落地 token/X-Auth-Token/password/captcha 掩码 有测试锚定
S4 非2xx/429/5xx/超时/业务code非1 按 ClassifyError 定级
S5 兼容现有行为 quiet 仅静默 stderr 不静默文件 默认无文件
S6 go test -race + golangci-lint + go vet 全绿

## 3 架构

pkg/logx 薄封装 Level/Format/File 解析与脱敏与 traceId 上下文；CLI 三旗标与 env 接线后组装 slog.Logger multi-writer；SDK 在 request.go 统一打点并经 context 透传 traceId 到 auth/session/file。

## 4 关键契约

- 新增 env NAZHI_LOG_LEVEL/FORMAT/FILE 优先级 flag 大于 env 大于默认
- --log-level 默认 warn --log-format 默认 text --log-file 默认空
- --verbose 兼容为 debug 仅当未显式传 --log-level 时生效
- --quiet 仅影响 stderr 文件仍写
- 脱敏黑名单 token/x-auth-token/authorization/password/passwd/captcha 大小写不敏感
- 截断 body 先掩码再截断默认256 header 掩码保留前后2字符便于排错
- 级别映射 2xx info 4xx warn 5xx error 超时与取消 warn 业务拒绝 warn 网络 error

## 5 文件清单

新增 pkg/logx/logx.go pkg/logx/redact.go
修改 pkg/client/client.go pkg/client/request.go pkg/client/auth.go pkg/client/session.go pkg/client/file.go cmd/nazhi/main.go cmd/nazhi/opt_builder.go cmd/nazhi/output.go
文档 docs/cli/README.md docs/sdk/README.md CLAUDE.md README.md CHANGELOG.md

## 6 测试

logx 单元 脱敏 multi-writer trace 上下文；request 统一打点；auth 脱敏；CLI flags/env 优先级与 quiet+file；端到端 trace 贯穿；回归现有 verbose/captcha 泄漏测试绿。

## 7 假设与天花板

无轮转 超过50MB 再引入 lumberjack 以 ponytail 注释标天花板；不引入 OTel 预留 trace_id 字段名对齐；脱敏仅顶层 key 正则足够当前契约。
