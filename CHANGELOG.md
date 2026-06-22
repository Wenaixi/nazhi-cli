# CHANGELOG

所有重要变更都会记录在此文件。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
项目遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

## [0.2.0] - 2026-06-22

🎉 **重大更新** — 跨平台 OCR + 进程级单例 + HAR 驱动测试 + Cookie 同步修复 + 完整文档体系。

### ✨ Features

#### 跨平台 OCR（5 平台）
- 5 平台 build tag 隔离的 `onnx_*.go` 嵌入文件（win/lin/mac × amd64/arm64）
- `ocr.GetDefault()` 进程级单例 + `sync.Mutex` 并发保护
- 99 次重试机制（同一图片）提高识别准确率
- 解压到磁盘目录供 `onnxruntime_go` 加载

#### 全自动验证码流程
- 简化 `Login()` 内部流程：InitSession → GetSchoolID → OCR → validate → 302/200 提取 token
- 优先处理 200 JSON 响应（HAR 验证），fallback 到 302 Location
- 移除所有手动/交互式验证码模式
- 自动 `syncCookieToken` 同步到 SSO + 业务域 Cookie

#### HAR 对齐的 4 步 Session 激活
- 步骤 1：GET / 初始化后端 Session
- 步骤 2：GET /api/studentInfo/getMenu（Referer: /homepage?token=xxx）
- 步骤 3：GET /api/studentInfo/getMenu（Referer: /home）
- 步骤 4：GET /api/studentInfo/getMyInfo（返回完整 51 字段 UserInfo）

#### UserInfo 51 字段
- 完整暴露 `getMyInfo` 返回数据
- `birthdayStr` 字符串化（Java LocalDate JSON 数组兼容）
- 移除自定义 `Birthday` 类型

#### 图片自动压缩预处理
- 任意格式 → JPG（PNG/BMP/WEBP/GIF 支持）
- 透明合成（flattenOnWhite）
- 质量级联 → 缩放级联
- 上限 5MB
- 全部在内存中完成，不写盘

#### CLI 环境变量支持
- `NAZHI_USERNAME` / `NAZHI_PASSWORD` / `NAZHI_TOKEN`
- `NAZHI_SSO_BASE` / `NAZHI_BASE_URL` / `NAZHI_UPLOAD_URL`
- `NAZHI_TIMEOUT`
- 命令行标志优先于环境变量（用 `flagChanged` 检测）
- `.env.example` 模板 + `.gitignore` 排除真实 `.env`

#### HAR 驱动集成测试
- 5 个 fixture 文件（task_flow、self_eval、military、class_meeting、labor）
- 6 个 HAR 驱动测试覆盖 FetchTasks、SubmitTask（4 种类型）、SubmitSelfEvaluation
- 真实环境 10 步端到端 `TestReal_FullChain`
- 4 个回归测试

#### 完整文档体系
- `docs/README.md` — 文档中心索引
- `docs/cli/README.md` — CLI 命令参考
- `docs/sdk/README.md` — Go SDK API 参考
- `docs/architecture.md` — 架构总览
- `docs/login-flow.md` — 登录流程详解
- `docs/cross-platform-ocr.md` — 跨平台 OCR 设计
- `docs/env-vars.md` — 环境变量参考
- `docs/har-testing.md` — HAR 驱动测试架构

### 🐛 Fixes

#### Security
- **历史凭据泄露已修复**（v0.1.0 之前）：通过 `git-filter-repo` 重写所有分支和 tag 历史
- **CLI `--token` Cookie 同步**：新增 `WithToken()` Option，CLI 传 token 时同时写 Header + Cookie
- **UploadFile 禁用重定向**：cleanClient.CheckRedirect 防止 302 跳转到攻击者主机

#### Bugs
- **Task.StartDate 字段错配**：从 `startDate`（数组）改为 `startDateStr`（字符串）
- **extractTokenFromLocation URL 解析**：从 `strings.Index` 改为 `net/url.Parse`，支持 fragment
- **session.go 步骤 1/2 Body 泄漏**：defer + io.Copy 模式
- **QuerySelfGradEvaluation 错误被吞**：所有路径失败时返回明确 error
- **FetchTasks 静默失败**：用 `c.logDebug` 记录（不破坏 API）
- **output.go stderr 编码失败**：加 `fmt.Fprintln` 兜底
- **ImagePrep 兜底大小检查**：避免返回超大文件
- **stdin 无 TTY 阻塞**：`isTerminalStdin()` 检测

#### Dead Code 清理
- 删除未使用的 4 个哨兵错误（ErrTokenExpired、ErrSessionExpired、ErrIncompleteResponse、ErrUnexpectedStatus）
- 删除未使用的类型（SchoolInfo、SessionInfo）
- 删除未使用的函数（EnforceCode、自定义 min）
- 删除 debug 工具目录（cmd/debuglogin/、cmd/reallogin/、cmd/getcaptcha/、cmd/ocrtest/）

### 🏗️ CI/CD

- 5 平台 native runner 矩阵（ubuntu-latest、ubuntu-22.04-arm64、macos-latest、windows-latest、windows-11-arm）
- 新增 `integration` Job：tag 发布时跑真实环境集成测试（需 secrets）
- 新增 `gofmt` 检查
- 新增 `go mod tidy` 验证
- 新增 SHA256 校验和
- 二进制 `--version` 验证步骤

### 📦 Build

- Go 1.26.1
- 单二进制分发（内嵌 OCR 模型 + onnxruntime）
- Makefile：`build` / `test` / `test-verbose` / `test-integration` / `lint` / `vet` / `fmt` / `release` / `clean`

## [0.1.0] - 2026-06-21

🎉 初始发布 — nazhi-cli：纳智综合评价自动化 CLI + Go SDK。

### Features

- **SSO 全自动登录** — InitSession → GetSchoolID → 验证码处理 → Login 全流程
- **内置 OCR 验证码识别** — ddddocr 引擎 + 模型已内嵌至二进制，无需运行时下载
- **学校 ID 查询** — 根据学号获取学校信息
- **业务 Session 激活** — 登录后激活目标平台 API Session
- **用户信息查询** — 获取当前用户 profile
- **任务管理** — 列出任务 + 提交任务（支持 `@file.json` 读取）
- **自我评价** — 提交评价 & 查询评价状态
- **文件上传** — 本地图片上传至目标平台
- **跨平台构建** — Linux / macOS / Windows 三平台二进制支持

### Tech

- Go 1.26 + cobra CLI 框架
- ddddocr（ONNX Runtime）嵌入式验证码识别
- 单二进制分发，零外部依赖
