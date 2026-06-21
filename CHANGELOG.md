# Changelog

## v0.1.0 (2026-06-21)

🎉 初始发布 — nazhi-cli：纳智综合评价自动化 CLI + Go SDK。

### Features

- **SSO 全自动登录** — InitSession → GetSchoolID → 验证码处理 → Login 全流程
- **内置 OCR 验证码识别** — ddddocr 引擎 + 模型已内嵌至二进制，无需运行时下载
  - `--ocr` 默认开启，同一验证码图片最多 OCR 重试 99 次
  - 支持 `--ocr=false` 交互式输入、`-c` 直接指定三种模式
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

### 后续规划

- 任务批量提交
- 自动评价 + 多账户并行
- 综合评价数据分析
